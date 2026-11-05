package config

import "testing"

func TestUINamespace(t *testing.T) {
	cases := []struct{ name, uiNamespace, namespace, want string }{
		{"platform namespace wins", "demo.acct", "other", "demo.acct"},
		{"connection namespace", "", "demo.acct", "demo.acct"},
		{"nothing set", "", "", "default"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("TEMPORAL_UI_NAMESPACE", c.uiNamespace)
			t.Setenv("TEMPORAL_NAMESPACE", c.namespace)
			cfg, err := FromEnv()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.UINamespace != c.want {
				t.Errorf("UINamespace = %q, want %q", cfg.UINamespace, c.want)
			}
		})
	}
}
