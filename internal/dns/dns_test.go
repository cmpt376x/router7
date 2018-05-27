package dns

import (
	"bytes"
	"net"
	"router7/internal/dhcp4d"
	"testing"

	"github.com/miekg/dns"
)

// TODO(later): upstream a dnstest.Recorder implementation
type recorder struct {
	response *dns.Msg
}

func (r *recorder) WriteMsg(m *dns.Msg) error {
	r.response = m
	return nil
}

func (r *recorder) LocalAddr() net.Addr       { return nil }
func (r *recorder) RemoteAddr() net.Addr      { return nil }
func (r *recorder) Write([]byte) (int, error) { return 0, nil }
func (r *recorder) Close() error              { return nil }
func (r *recorder) TsigStatus() error         { return nil }
func (r *recorder) TsigTimersOnly(bool)       {}
func (r *recorder) Hijack()                   {}

func TestNXDOMAIN(t *testing.T) {
	r := &recorder{}
	s := NewServer("localhost:0", "lan")
	m := new(dns.Msg)
	m.SetQuestion("foo.invalid.", dns.TypeA)
	s.handleRequest(r, m)
	if got, want := r.response.MsgHdr.Rcode, dns.RcodeNameError; got != want {
		t.Fatalf("unexpected rcode: got %v, want %v", got, want)
	}
}

func TestResolveError(t *testing.T) {
	r := &recorder{}
	s := NewServer("localhost:0", "lan")
	s.upstream = "266.266.266.266:53"
	m := new(dns.Msg)
	m.SetQuestion("foo.invalid.", dns.TypeA)
	s.handleRequest(r, m)
	if r.response != nil {
		t.Fatalf("r.response unexpectedly not nil: %v", r.response)
	}
}

func TestDHCP(t *testing.T) {
	r := &recorder{}
	s := NewServer("localhost:0", "lan")
	s.SetLeases([]dhcp4d.Lease{
		{
			Hostname: "xps",
			Addr:     net.IP{192, 168, 42, 23},
		},
	})
	m := new(dns.Msg)
	m.SetQuestion("xps.lan.", dns.TypeA)
	s.handleRequest(r, m)
	if got, want := len(r.response.Answer), 1; got != want {
		t.Fatalf("unexpected number of answers: got %d, want %d", got, want)
	}
	a := r.response.Answer[0]
	if _, ok := a.(*dns.A); !ok {
		t.Fatalf("unexpected response type: got %T, want dns.A", a)
	}
	if got, want := a.(*dns.A).A.To4(), (net.IP{192, 168, 42, 23}); !bytes.Equal(got, want) {
		t.Fatalf("unexpected response IP: got %v, want %v", got, want)
	}
}

func TestDHCPReverse(t *testing.T) {
	r := &recorder{}
	s := NewServer("localhost:0", "lan")
	s.SetLeases([]dhcp4d.Lease{
		{
			Hostname: "xps",
			Addr:     net.IP{192, 168, 42, 23},
		},
	})
	m := new(dns.Msg)
	m.SetQuestion("23.42.168.192.in-addr.arpa.", dns.TypePTR)
	s.handleRequest(r, m)
	if got, want := len(r.response.Answer), 1; got != want {
		t.Fatalf("unexpected number of answers: got %d, want %d", got, want)
	}
	a := r.response.Answer[0]
	if _, ok := a.(*dns.PTR); !ok {
		t.Fatalf("unexpected response type: got %T, want dns.A", a)
	}
	if got, want := a.(*dns.PTR).Ptr, "xps.lan."; got != want {
		t.Fatalf("unexpected response record: got %q, want %q", got, want)
	}
}

// TODO: multiple questions