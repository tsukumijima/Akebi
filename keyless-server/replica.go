package main

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"time"
)

type replicaRequest struct {
	ChallengeType string `json:"challenge_type"`
	ChallengeID   string `json:"challenge_id"`
}

type replicaResponse []string

func replicaClient(typ, id string) []string {
	if config.Replica == "" {
		return nil
	}

	conn, err := net.Dial("udp", config.Replica)
	if err != nil {
		log.Println(err)
		return nil
	}
	conn.SetDeadline(time.Now().Add(time.Second))
	if err != nil {
		log.Println(err)
		return nil
	}

	var buf [512]byte
	var req = replicaRequest{ChallengeType: typ, ChallengeID: id}
	if json, err := json.Marshal(req); err != nil {
		log.Println(err)
		return nil
	} else if len(json) > len(buf) {
		log.Println("request size too long", len(json))
		return nil
	} else {
		for i := copy(buf[:], json); i < len(buf); i++ {
			buf[i] = ' '
		}
	}
	if _, err := conn.Write(buf[:]); err != nil {
		log.Println(err)
		return nil
	}

	var res replicaResponse
	if n, err := conn.Read(buf[:]); err != nil {
		log.Println(err)
		return nil
	} else if err := json.Unmarshal(buf[:n], &res); err != nil {
		log.Println(err)
		return nil
	}
	return res
}

func replicaServe(conn net.PacketConn) error {
	var buf [512]byte
	for {
		var nerr net.Error
		n, addr, err := conn.ReadFrom(buf[:])
		if errors.As(err, &nerr) && !nerr.Temporary() && !nerr.Timeout() {
			return err
		}
		if err != nil {
			log.Println(err)
			continue
		}

		var req replicaRequest
		if err := json.Unmarshal(buf[:n], &req); err != nil {
			log.Println(err)
			continue
		}

		var res replicaResponse = solvers.GetLocalAuthorizations(req.ChallengeType, req.ChallengeID)
		if json, err := json.Marshal(res); err != nil {
			log.Println(err)
		} else if len(json) > n {
			log.Println("response longer than request", len(json))
		} else if _, err := conn.WriteTo(json, addr); err != nil {
			log.Println(err)
		}
	}
}
