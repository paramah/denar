package smtp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"io"
	"math/big"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paramah/denar/internal/usecase"
)

// fakeServer is a minimal SMTP server for one message per connection.
type fakeServer struct {
	t        *testing.T
	ln       net.Listener
	tls      *tls.Config
	implicit bool // TLS from the start
	starttls bool // offers STARTTLS
	auth     bool // offers AUTH PLAIN
	rejectTo bool

	mu       sync.Mutex
	from, to string
	authLine string
	data     string
	upgraded bool
}

func newFakeServer(t *testing.T, cfg *tls.Config, implicit, starttls, auth bool) *fakeServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if implicit {
		ln = tls.NewListener(ln, cfg)
	}
	s := &fakeServer{t: t, ln: ln, tls: cfg, implicit: implicit, starttls: starttls, auth: auth}
	t.Cleanup(func() { ln.Close() })
	go s.serve()
	return s
}

func (s *fakeServer) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *fakeServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeServer) handle(conn net.Conn) {
	defer conn.Close()
	tp := textproto.NewConn(conn)
	tp.PrintfLine("220 fake ESMTP")
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.Fields(line + " x")[0])
		switch cmd {
		case "EHLO", "HELO":
			lines := []string{"fake"}
			if s.starttls && !s.upgraded {
				lines = append(lines, "STARTTLS")
			}
			if s.auth {
				lines = append(lines, "AUTH PLAIN")
			}
			for i, l := range lines {
				sep := "-"
				if i == len(lines)-1 {
					sep = " "
				}
				tp.PrintfLine("250%s%s", sep, l)
			}
		case "STARTTLS":
			tp.PrintfLine("220 go ahead")
			tlsConn := tls.Server(conn, s.tls)
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			s.mu.Lock()
			s.upgraded = true
			s.mu.Unlock()
			conn = tlsConn
			tp = textproto.NewConn(conn)
		case "AUTH":
			s.mu.Lock()
			s.authLine = line
			s.mu.Unlock()
			tp.PrintfLine("235 ok")
		case "MAIL":
			s.mu.Lock()
			s.from = line
			s.mu.Unlock()
			tp.PrintfLine("250 ok")
		case "RCPT":
			if s.rejectTo {
				tp.PrintfLine("550 no such user")
				continue
			}
			s.mu.Lock()
			s.to = line
			s.mu.Unlock()
			tp.PrintfLine("250 ok")
		case "DATA":
			tp.PrintfLine("354 go")
			data, err := io.ReadAll(tp.DotReader())
			if err != nil {
				return
			}
			s.mu.Lock()
			s.data = string(data)
			s.mu.Unlock()
			tp.PrintfLine("250 queued")
		case "QUIT":
			tp.PrintfLine("221 bye")
			return
		default:
			tp.PrintfLine("502 unknown")
		}
	}
}

func (s *fakeServer) got() (from, to, auth, data string, upgraded bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.from, s.to, s.authLine, s.data, s.upgraded
}

// testTLS returns a server config with a self-signed certificate for
// 127.0.0.1 and a client config trusting it.
func testTLS(t *testing.T) (server, client *tls.Config) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}},
		&tls.Config{RootCAs: pool, ServerName: "127.0.0.1"}
}

func newMailer(t *testing.T, s *fakeServer, mode string, client *tls.Config, user string) *Mailer {
	t.Helper()
	m, err := New(Config{Host: "127.0.0.1", Port: s.port(), Username: user, Password: "pa55", From: "Denar <denar@example.com>",
		TLS: mode, TLSConfig: client, Timeout: 5 * time.Second, Now: func() time.Time { return time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func wantMessage(t *testing.T, data, subject, bodyPart string) {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(data))
	if err != nil {
		t.Fatalf("message %q: %v", data, err)
	}
	got, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if got != subject || msg.Header.Get("From") != `"Denar" <denar@example.com>` || msg.Header.Get("Content-Type") != "text/plain; charset=utf-8" ||
		msg.Header.Get("Date") != "Wed, 21 Oct 2026 09:00:00 +0000" || !strings.HasSuffix(msg.Header.Get("Message-ID"), "@example.com>") {
		t.Errorf("headers = %v (subject %q)", msg.Header, got)
	}
	body, _ := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if !strings.Contains(string(body), bodyPart) {
		t.Errorf("body = %q, want %q in it", body, bodyPart)
	}
}

func TestPlain(t *testing.T) {
	s := newFakeServer(t, nil, false, false, false)
	m := newMailer(t, s, None, nil, "")
	err := m.Send(context.Background(), usecase.ChannelTarget{Config: map[string]string{"address": "ola@example.com"}},
		usecase.OutgoingMessage{Title: "Odnowienie: Netflix jutro", Body: "Netflix odnowi się 31.10.2026: 43.00 zł.",
			URL: "https://denar.example.com/h/h1/subscriptions/s1", HouseholdName: "Dom"})
	if err != nil {
		t.Fatal(err)
	}
	from, to, auth, data, _ := s.got()
	if from != "MAIL FROM:<denar@example.com> BODY=8BITMIME" && from != "MAIL FROM:<denar@example.com>" {
		t.Errorf("from = %q", from)
	}
	if to != "RCPT TO:<ola@example.com>" || auth != "" {
		t.Errorf("to = %q, auth = %q", to, auth)
	}
	wantMessage(t, data, "Odnowienie: Netflix jutro", "Netflix odnowi się 31.10.2026: 43.00 zł.\n\nhttps://denar.example.com/h/h1/subscriptions/s1\n\nGospodarstwo: Dom")
}

func TestStartTLSWithAuth(t *testing.T) {
	server, client := testTLS(t)
	s := newFakeServer(t, server, false, true, true)
	m := newMailer(t, s, StartTLS, client, "denar")
	if err := m.SendMail(context.Background(), "grace@example.com", "Zaproszenie do gospodarstwa Dom", "Link: https://x/invite#abc"); err != nil {
		t.Fatal(err)
	}
	_, to, auth, data, upgraded := s.got()
	if !upgraded || to != "RCPT TO:<grace@example.com>" {
		t.Errorf("upgraded = %v, to = %q", upgraded, to)
	}
	want := "AUTH PLAIN " + base64.StdEncoding.EncodeToString([]byte("\x00denar\x00pa55"))
	if auth != want {
		t.Errorf("auth = %q, want %q", auth, want)
	}
	wantMessage(t, data, "Zaproszenie do gospodarstwa Dom", "Link: https://x/invite#abc")
}

func TestImplicitTLS(t *testing.T) {
	server, client := testTLS(t)
	s := newFakeServer(t, server, true, false, true)
	m := newMailer(t, s, TLS, client, "denar")
	if err := m.SendMail(context.Background(), "grace@example.com", "Test", "Treść"); err != nil {
		t.Fatal(err)
	}
	if _, _, auth, _, _ := s.got(); auth == "" {
		t.Error("no AUTH over TLS")
	}
}

func TestErrors(t *testing.T) {
	s := newFakeServer(t, nil, false, false, false)
	if err := newMailer(t, s, StartTLS, nil, "").SendMail(context.Background(), "a@example.com", "x", "y"); err == nil ||
		!strings.Contains(err.Error(), "STARTTLS") {
		t.Errorf("no STARTTLS = %v", err)
	}
	s.rejectTo = true
	if err := newMailer(t, s, None, nil, "").SendMail(context.Background(), "a@example.com", "x", "y"); err == nil ||
		!strings.Contains(err.Error(), "550") {
		t.Errorf("rejected = %v", err)
	}
	if err := newMailer(t, s, None, nil, "").SendMail(context.Background(), "not an address", "x", "y"); err == nil {
		t.Error("bad address passed")
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	m, _ := New(Config{Host: "127.0.0.1", Port: port, From: "denar@example.com", TLS: None, Password: "pa55"})
	if err := m.SendMail(context.Background(), "a@example.com", "x", "y"); err == nil || strings.Contains(err.Error(), "pa55") {
		t.Errorf("closed port = %v", err)
	}
	for _, cfg := range []Config{{From: "a@example.com"}, {Host: "h", From: "nope"}, {Host: "h", From: "a@example.com", TLS: "ssl"}} {
		if _, err := New(cfg); err == nil {
			t.Errorf("New(%+v) passed", cfg)
		}
	}
	if m, _ := New(Config{Host: "h", From: "a@example.com", TLS: TLS}); m.cfg.Port != 465 {
		t.Errorf("TLS port = %d", m.cfg.Port)
	}
	if m, _ := New(Config{Host: "h", From: "a@example.com"}); m.cfg.Port != 587 || m.cfg.TLS != StartTLS {
		t.Errorf("default = %+v", m.cfg)
	}
}
