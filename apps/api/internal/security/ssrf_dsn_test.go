package security

import (
	"errors"
	"net"
	"testing"
)

func TestValidatePostgresDSNRefusesInternalHosts(t *testing.T) {
	t.Setenv("ALLOW_INTERNAL_SSRF", "")
	t.Setenv("IS_DEVELOPMENT", "")
	t.Setenv("APP_ENV", "production")

	for _, dsn := range []string{
		"postgres://postgres:postgres@postgres:5432/prism",  // compose service name, does not resolve from here either
		"postgres://postgres:postgres@localhost:5432/prism", // local alias
		"postgres://postgres:postgres@127.0.0.1:5432/prism", // loopback
		"postgres://u:p@10.0.0.5:5432/db",                   // private
		"postgres://u:p@169.254.169.254/db",                 // metadata
		"postgres://u:p@db.internal:5432/db",                // internal suffix
		"postgresql://u:p@[::1]:5432/db",                    // IPv6 loopback
		"host=localhost user=postgres password=x dbname=db", // key=value form
		"postgres:///db",             // socket, no host
		"mysql://u:p@example.com/db", // wrong scheme
		"",
	} {
		if err := ValidatePostgresDSN(dsn); err == nil {
			t.Errorf("%q accepted", dsn)
		}
	}
	if err := ValidatePostgresDSN("postgres:///db"); !errors.Is(err, ErrDSNHostRequired) {
		t.Errorf("socket DSN: err = %v, want ErrDSNHostRequired", err)
	}
}

func TestValidatePostgresDSNAcceptsAPublicHost(t *testing.T) {
	t.Setenv("ALLOW_INTERNAL_SSRF", "")
	t.Setenv("APP_ENV", "production")
	if _, err := net.LookupIP("example.com"); err != nil {
		t.Skipf("no DNS: %v", err)
	}
	if err := ValidatePostgresDSN("postgres://u:p@example.com:5432/db?sslmode=require"); err != nil {
		t.Fatalf("public host rejected: %v", err)
	}
}

// The bypass used to work with ALLOW_INTERNAL_SSRF alone, so one stray
// variable could switch the guard off in production.
func TestBypassNeedsDevelopmentEnvironment(t *testing.T) {
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")
	t.Setenv("IS_DEVELOPMENT", "true")
	t.Setenv("APP_ENV", "production")
	if err := ValidateOutboundURL("http://127.0.0.1:8080"); err == nil {
		t.Fatal("loopback accepted in production with the bypass flags set")
	}
	if err := ValidatePostgresDSN("postgres://u:p@127.0.0.1/db"); err == nil {
		t.Fatal("loopback DSN accepted in production with the bypass flags set")
	}

	t.Setenv("APP_ENV", "development")
	if err := ValidateOutboundURL("http://127.0.0.1:8080"); err != nil {
		t.Fatalf("development bypass not honoured: %v", err)
	}
}
