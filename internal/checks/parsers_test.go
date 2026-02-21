package checks

import "testing"

func TestParsePing(t *testing.T) {
	raw := `64 bytes from 1.1.1.1: icmp_seq=0 ttl=57 time=10.1 ms
64 bytes from 1.1.1.1: icmp_seq=1 ttl=57 time=11.2 ms
64 bytes from 1.1.1.1: icmp_seq=2 ttl=57 time=29.8 ms
64 bytes from 1.1.1.1: icmp_seq=3 ttl=57 time=30.1 ms
10 packets transmitted, 10 packets received, 0.0% packet loss
round-trip min/avg/max/stddev = 10.000/20.000/30.000/5.000 ms`
	loss, avg, jitter, p95 := parsePing(raw)
	if loss != 0 || avg != 20 || jitter != 5 {
		t.Fatalf("unexpected parse: %v %v %v", loss, avg, jitter)
	}
	if p95 <= 0 {
		t.Fatalf("expected p95 > 0, got %v", p95)
	}
}

func TestParseDigMS(t *testing.T) {
	if got := parseDigMS(";; Query time: 43 msec"); got != 43 {
		t.Fatalf("got %v", got)
	}
}

func TestParseCurlTimings(t *testing.T) {
	m := parseCurlTimings("dns:0.01 connect:0.02 tls:0.03 ttfb:0.04 total:0.05")
	if m["total"] != 50 {
		t.Fatalf("got %v", m["total"])
	}
}

func TestParseMTRSummary(t *testing.T) {
	hops, nearLoss := parseMTRSummary("1.|-- a 0.0%\n2.|-- b 1.5%\n3.|-- c 2.1%")
	if hops != 3 || nearLoss != 2.1 {
		t.Fatalf("unexpected mtr summary hops=%d nearLoss=%v", hops, nearLoss)
	}
}

func TestParseTracerouteSummary(t *testing.T) {
	hops, timeoutHops := parseTracerouteSummary("traceroute to 1.1.1.1\n 1  a  1.0 ms 1.1 ms 1.2 ms\n 2  * * *\n 3  c  5.0 ms 5.1 ms 5.2 ms")
	if hops != 3 || timeoutHops != 1 {
		t.Fatalf("unexpected traceroute summary hops=%d timeout=%d", hops, timeoutHops)
	}
}
