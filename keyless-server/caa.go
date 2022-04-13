package main

import (
	"bytes"

	"golang.org/x/net/dns/dnsmessage"
)

func convertTXTtoCAA(in []byte) []byte {
	txt2caa := func(rec []byte) (skip int, data []byte) {
		// len("\x00\x10\x00\x01\x00\x00\x00\x00\x00\x0c\x0b0 issue \";\"")
		if len(rec) < 22 {
			return 0, nil
		}
		if (rec[0] != 0 || rec[1] != byte(dnsmessage.TypeTXT)) ||
			rec[2] != 0 || rec[3] != byte(dnsmessage.ClassINET) {
			return 0, nil
		}
		// TXT record has a single value
		if rec[8] != 0 || int(rec[9]) != int(rec[10])+1 {
			return 0, nil
		}

		rec = rec[:11+int(rec[10])]
		val := rec[11:]

		// len("0 issue \";\"")
		if len(val) < 11 {
			return 0, nil
		}

		// TXT record starts with 0 or 1
		var flags byte
		switch val[0] {
		case '0':
		case '1':
			flags = 128
		default:
			return 0, nil
		}

		// TXT record ends with "
		if len := len(val) - 1; val[len] != '"' {
			return 0, nil
		} else {
			val = val[:len]
		}

		var tag string
		switch {
		case bytes.HasPrefix(val[1:], []byte(` issue "`)):
			tag = "issue"
			val = val[1+len(` issue "`):]
		case bytes.HasPrefix(val[1:], []byte(` issuewild "`)):
			tag = "issuewild"
			val = val[1+len(` issuewild "`):]
		case bytes.HasPrefix(val[1:], []byte(` iodef "`)):
			tag = "iodef"
			val = val[1+len(` iodef "`):]
		default:
			return 0, nil
		}

		data = make([]byte, 0, len(rec)-4) // removed 2 spaces, 2 quotes
		data = append(data, 1, 1)          // TypeCAA
		data = append(data, 0, byte(dnsmessage.ClassINET))
		data = append(data, rec[4:8]...) // TTL
		data = append(data, 0, rec[9]-4) // removed 2 spaces, 2 quotes
		data = append(data, flags, byte(len(tag)))
		data = append(data, tag...)
		data = append(data, val...)
		return len(rec), data
	}

	out := make([]byte, 0, len(in))
	for i := 0; i < len(in); {
		n, rec := txt2caa(in[i:])
		if rec != nil {
			out = append(out, rec...)
			i += n
		} else {
			out = append(out, in[i])
			i += 1
		}
	}
	return out
}
