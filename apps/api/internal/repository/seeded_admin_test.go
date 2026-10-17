package repository

import "testing"

// Migration 108 removes the admin account that 008 seeded with a committed
// password; the shared providers it used to own must survive.
func TestSeededAdminAccountIsGone(t *testing.T) {
	db := migratedDatabase(t)
	const admin = "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
	for _, q := range []string{
		`SELECT count(*) FROM "user" WHERE id = '` + admin + `' OR email = 'admin@roozylabs.com'`,
		`SELECT count(*) FROM account WHERE user_id = '` + admin + `' OR account_id = 'admin@roozylabs.com'`,
		`SELECT count(*) FROM routing_policies WHERE user_id = '` + admin + `'`,
		`SELECT count(*) FROM budgets WHERE user_id = '` + admin + `'`,
		`SELECT count(*) FROM credentials WHERE user_id = '` + admin + `'`,
	} {
		var n int
		if err := db.QueryRow(q).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		if n != 0 {
			t.Fatalf("%s = %d, want 0", q, n)
		}
	}
	var shared int
	if err := db.QueryRow(`SELECT count(*) FROM providers WHERE user_id IS NULL AND enabled`).Scan(&shared); err != nil {
		t.Fatal(err)
	}
	if shared < 3 {
		t.Fatalf("shared system providers = %d, want the seeded ones to survive", shared)
	}
}
