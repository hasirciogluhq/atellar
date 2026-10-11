package host

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type recorder struct {
	calls [][]string
	fail  string
}

func (r *recorder) run(args ...string) error {
	r.calls = append(r.calls, args)
	if r.fail != "" && args[0] == r.fail {
		return errors.New(r.fail + " failed")
	}
	return nil
}

func TestWriteFile(t *testing.T) {
	h := Host{Root: t.TempDir()}
	if err := h.WriteFile("/etc/atellar/api.env", []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(h.Root, "etc/atellar/api.env"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", st.Mode().Perm())
	}
	// Rewriting tightens the mode of an existing file too.
	if err := h.WriteFile("/etc/atellar/api.env", []byte("A=2\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	st, _ = os.Stat(filepath.Join(h.Root, "etc/atellar/api.env"))
	if st.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %v, want 0640", st.Mode().Perm())
	}
}

func TestRequireFile(t *testing.T) {
	h := Host{Root: t.TempDir()}
	err := h.RequireFile(Binary, "install the release first")
	if err == nil || !strings.Contains(err.Error(), "install the release first") {
		t.Fatalf("err = %v, want hint", err)
	}
	if err := h.WriteFile(Binary, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := h.RequireFile(Binary, ""); err != nil {
		t.Fatal(err)
	}
}

func TestInstallUnit(t *testing.T) {
	cases := []struct {
		name  string
		start bool
		fail  string
		calls [][]string
		err   bool
	}{
		{"enable only", false, "", [][]string{{"daemon-reload"}, {"enable", AgentUnit}}, false},
		{"enable and start", true, "", [][]string{{"daemon-reload"}, {"enable", AgentUnit}, {"restart", AgentUnit}}, false},
		{"reload fails", true, "daemon-reload", [][]string{{"daemon-reload"}}, true},
		{"enable fails", true, "enable", [][]string{{"daemon-reload"}, {"enable", AgentUnit}}, true},
		{"start fails", true, "restart", [][]string{{"daemon-reload"}, {"enable", AgentUnit}, {"restart", AgentUnit}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{fail: tc.fail}
			h := Host{Root: t.TempDir(), Systemctl: rec.run}
			if h.UnitInstalled(AgentUnit) {
				t.Fatal("unit reported installed before install")
			}
			err := h.InstallUnit(AgentUnit, AgentUnitFile(), tc.start)
			if (err != nil) != tc.err {
				t.Fatalf("err = %v, want error=%v", err, tc.err)
			}
			if !reflect.DeepEqual(rec.calls, tc.calls) {
				t.Fatalf("systemctl calls = %v, want %v", rec.calls, tc.calls)
			}
			if !h.UnitInstalled(AgentUnit) {
				t.Fatal("unit file not written")
			}
		})
	}
}

func TestRestart(t *testing.T) {
	rec := &recorder{}
	if err := (Host{Systemctl: rec.run}).Restart(APIUnit); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rec.calls, [][]string{{"restart", APIUnit}}) {
		t.Fatalf("calls = %v", rec.calls)
	}
}

func TestUnitFiles(t *testing.T) {
	api := APIUnitFile("/etc/atellar/api.env")
	for _, want := range []string{"EnvironmentFile=/etc/atellar/api.env", "ExecStart=" + Binary + " server run", "WantedBy=multi-user.target"} {
		if !strings.Contains(api, want) {
			t.Errorf("API unit missing %q", want)
		}
	}
	agent := AgentUnitFile()
	for _, want := range []string{"ExecStart=" + Binary + " agent run", "Requires=containerd.service"} {
		if !strings.Contains(agent, want) {
			t.Errorf("agent unit missing %q", want)
		}
	}
}
