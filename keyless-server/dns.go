package main

import (
	"errors"
	"net"
	"strings"

	"golang.org/x/net/dns/dnsmessage"
)

var (
	nameserver dnsmessage.Name
	cname      dnsmessage.Name
)

func dnsConfig() error {
	var err error
	nameserver, err = dnsmessage.NewName(config.Nameserver + ".")
	if err == nil && config.CName != "" {
		cname, err = dnsmessage.NewName(config.CName + ".")
	}
	return err
}

func dnsServe(conn net.PacketConn) error {
	var buf [512]byte
	for {
		var nerr net.Error
		n, addr, err := conn.ReadFrom(buf[:])
		if errors.As(err, &nerr) && !nerr.Temporary() && !nerr.Timeout() {
			return err
		}
		if err != nil {
			logError(err)
			continue
		}

		var parser dnsmessage.Parser
		header, err := parser.Start(buf[:n])
		if err != nil {
			logError(err)
			continue
		}

		var res response
		res.header.ID = header.ID
		res.header.Response = true
		res.header.OpCode = header.OpCode
		res.header.Authoritative = true
		res.header.RecursionDesired = header.RecursionDesired

		// only QUERY is implemented
		if header.OpCode != 0 {
			res.header.RCode = dnsmessage.RCodeNotImplemented
			logError(res.send(conn, addr, buf[:0]))
			continue
		}

		question, err := parser.Question()
		// refuse zero questions
		if err == dnsmessage.ErrSectionDone {
			res.header.RCode = dnsmessage.RCodeRefused
			logError(res.send(conn, addr, buf[:0]))
			continue
		}
		// report error
		if err != nil {
			res.header.RCode = dnsmessage.RCodeFormatError
			logError(res.send(conn, addr, buf[:0]))
			continue
		}
		// answer the first question only, ingore everything else
		res.header.RCode = res.answerQuestion(question)
		logError(res.send(conn, addr, buf[:0]))
	}
}

type response struct {
	header   dnsmessage.Header
	question dnsmessage.Question
	answer   func(*dnsmessage.Builder) error
}

func (r *response) answerQuestion(question dnsmessage.Question) dnsmessage.RCode {
	// ANY is not implemented
	if question.Type == dnsmessage.TypeALL {
		return dnsmessage.RCodeNotImplemented
	}

	// refuse everything outside our zone
	name := strings.TrimSuffix(strings.ToLower(question.Name.String()), ".")
	if n := strings.TrimSuffix(name, config.Domain); len(n) != len(name) {
		switch {
		case len(n) == 0:
			name = ""
		case n[len(n)-1] == '.':
			name = n[:len(n)-1]
		default:
			return dnsmessage.RCodeRefused
		}
	} else {
		return dnsmessage.RCodeRefused
	}

	// otherwise, answer the question
	r.question = question

	header := dnsmessage.ResourceHeader{
		Name:  question.Name,
		Class: dnsmessage.ClassINET,
	}

	// apex domain, must have SOA and NS
	if name == "" {
		switch {
		case question.Type == dnsmessage.TypeSOA:
			r.answer = func(b *dnsmessage.Builder) error {
				return b.SOAResource(getAuthority(r.question.Name))
			}

		case question.Type == dnsmessage.TypeNS:
			header.TTL = 7 * 86400 // 7 days
			r.answer = func(b *dnsmessage.Builder) error {
				return b.NSResource(header, dnsmessage.NSResource{NS: nameserver})
			}

		// CAA allows Let's Encrypt wildcards, denies everything else
		// this prevents DoS by exausting Let's Encrypt quotas
		// dnsmessage does not support CAA records: use TXT, convert to CAA later
		case question.Type == 257: // CAA
			header.TTL = 7 * 86400 // 7 days
			r.answer = func(b *dnsmessage.Builder) error {
				err := b.TXTResource(header, dnsmessage.TXTResource{TXT: []string{`0 issue ";"`}})
				if err != nil {
					return err
				}
				return b.TXTResource(header, dnsmessage.TXTResource{TXT: []string{`0 issuewild "letsencrypt.org"`}})
			}

		// CNAME is not RFC compliant, but mostly works:
		// https://blog.cloudflare.com/zone-apex-naked-domain-root-domain-cname-supp/
		case cname.Length != 0:
			header.TTL = 5 * 60 // 5 minutes
			r.answer = func(b *dnsmessage.Builder) error {
				return b.CNAMEResource(header, dnsmessage.CNAMEResource{CNAME: cname})
			}
		}
		return dnsmessage.RCodeSuccess
	}

	// Let's Encrypt challenge
	if name == "_acme-challenge" {
		if question.Type == dnsmessage.TypeTXT {
			challenges := solvers.GetDNSChallenges(config.Domain)
			if len(challenges) > 0 {
				header.TTL = 5 * 60 // 5 minutes
				r.answer = func(b *dnsmessage.Builder) error {
					return b.TXTResource(header, dnsmessage.TXTResource{
						TXT: challenges,
					})
				}
			}
		}
		return dnsmessage.RCodeSuccess
	}

	// NXDOMAIN multi-level subdomains
	if strings.ContainsRune(name, '.') {
		return dnsmessage.RCodeNameError
	}

	// finally, IP addresses
	ipv4 := getIPv4(name)
	ipv6 := getIPv6(name)
	if ipv4 != nil || ipv6 != nil {
		switch question.Type {
		case dnsmessage.TypeA:
			if ipv4 != nil {
				res := dnsmessage.AResource{}
				copy(res.A[:], ipv4)
				header.TTL = 7 * 86400 // 7 days

				r.answer = func(b *dnsmessage.Builder) error {
					return b.AResource(header, res)
				}
			}

		case dnsmessage.TypeAAAA:
			if ipv6 != nil {
				res := dnsmessage.AAAAResource{}
				copy(res.AAAA[:], ipv6)
				header.TTL = 7 * 86400 // 7 days

				r.answer = func(b *dnsmessage.Builder) error {
					return b.AAAAResource(header, res)
				}
			}
		}
		return dnsmessage.RCodeSuccess
	}

	// NXDOMAIN everything else
	return dnsmessage.RCodeNameError
}

func (r *response) send(conn net.PacketConn, addr net.Addr, buf []byte) error {
	builder := dnsmessage.NewBuilder(buf, r.header)
	builder.EnableCompression()

	err := r.sendQuestion(&builder)
	if err != nil {
		return err
	}

	err = r.sendAnswer(&builder)
	if err != nil {
		return err
	}

	err = r.sendAuthority(&builder)
	if err != nil {
		return err
	}

	out, err := builder.Finish()
	if err != nil {
		return err
	}

	out = convertTXTtoCAA(out)

	// truncate
	if len(out) > 512 {
		out = out[:512]
		out[2] |= 2
	}

	_, err = conn.WriteTo(out, addr)
	return err
}

func (r *response) sendQuestion(builder *dnsmessage.Builder) error {
	if r.question.Type == 0 {
		return nil
	}

	err := builder.StartQuestions()
	if err != nil {
		return err
	}

	return builder.Question(r.question)
}

func (r *response) sendAnswer(builder *dnsmessage.Builder) error {
	if r.answer == nil {
		return nil
	}

	err := builder.StartAnswers()
	if err != nil {
		return err
	}

	return r.answer(builder)
}

func (r *response) sendAuthority(builder *dnsmessage.Builder) error {
	// send SOA for NOERROR or NXDOMAIN with no answer
	if r.header.RCode != dnsmessage.RCodeSuccess && r.header.RCode != dnsmessage.RCodeNameError {
		return nil
	}
	if r.answer != nil {
		return nil
	}

	err := builder.StartAuthorities()
	if err != nil {
		return err
	}

	return builder.SOAResource(getAuthority(r.question.Name))
}

func getIPv4(name string) net.IP {
	if name == "local" || name == "localhost" {
		return net.IPv4(127, 0, 0, 1).To4()
	}

	name = strings.ReplaceAll(name, "-", ".")
	return net.ParseIP(string(name)).To4()
}

func getIPv6(name string) net.IP {
	if name == "local" || name == "localhost" {
		return net.IPv6loopback
	}

	name = strings.ReplaceAll(name, "-", ":")
	return net.ParseIP(string(name))
}

func getAuthority(name dnsmessage.Name) (dnsmessage.ResourceHeader, dnsmessage.SOAResource) {
	return dnsmessage.ResourceHeader{
			Name:  name,
			Class: dnsmessage.ClassINET,
			TTL:   7 * 86400, // 7 days
		}, dnsmessage.SOAResource{
			NS:   nameserver,
			MBox: nameserver,
			// https://www.ripe.net/publications/docs/ripe-203
			Refresh: 86400,
			Retry:   7200,
			Expire:  3600000,
			MinTTL:  3600,
		}
}
