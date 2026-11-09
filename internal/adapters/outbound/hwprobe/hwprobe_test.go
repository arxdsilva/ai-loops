package hwprobe

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/hw"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseMeminfo(t *testing.T) {
	tot, av, err := parseMeminfo(fixture(t, "meminfo.txt"))
	if err != nil || tot != 16384000*1024 || av != 11744051*1024 {
		t.Fatal(tot, av, err)
	}
	if _, _, err := parseMeminfo("garbage"); err == nil {
		t.Fatal("want error")
	}
}

func TestParseLoadavg(t *testing.T) {
	if v, err := parseLoadavg(fixture(t, "loadavg.txt")); err != nil || v != 1.52 {
		t.Fatal(v, err)
	}
	if _, err := parseLoadavg(""); err == nil {
		t.Fatal("want error")
	}
	if v, err := parseDarwinLoadavg("{ 2.50 1.20 1.10 }\n"); err != nil || v != 2.5 {
		t.Fatal(v, err)
	}
}

func TestParseCgroup(t *testing.T) {
	if c, err := parseCPUMax(fixture(t, "cpu.max.unlimited")); err != nil || c != 0 {
		t.Fatal(c, err)
	}
	if c, err := parseCPUMax(fixture(t, "cpu.max.limited")); err != nil || c != 1.5 {
		t.Fatal(c, err)
	}
	if _, err := parseCPUMax("x y"); err == nil {
		t.Fatal("want error")
	}
	if c, err := quotaCores("-1", "100000"); err != nil || c != 0 {
		t.Fatal(c, err)
	}
	if _, err := quotaCores("100", "0"); err == nil {
		t.Fatal("want error")
	}
	for name, want := range map[string]uint64{"memory.max.unlimited": 0, "memory.max.limited": 2 << 30, "memory.limit_in_bytes.huge": 0} {
		if v, err := parseMemLimit(fixture(t, name)); err != nil || v != want {
			t.Errorf("%s: %d %v", name, v, err)
		}
	}
	if _, err := parseMemLimit(""); err == nil {
		t.Fatal("want error")
	}
}

func TestParseVMStat(t *testing.T) {
	v, err := parseVMStat(fixture(t, "vm_stat.txt"))
	if want := uint64(106000 * 16384); err != nil || v != want {
		t.Fatal(v, err)
	}
	if _, err := parseVMStat("nope"); err == nil {
		t.Fatal("want error")
	}
	if v, err := parseSysctlUint("17179869184\n"); err != nil || v != 17179869184 {
		t.Fatal(v, err)
	}
}

func TestParseWindowsMem(t *testing.T) {
	tot, av, err := parseWindowsMem(fixture(t, "windows_mem.json"))
	if err != nil || tot != 16777216*1024 || av != 8388608*1024 {
		t.Fatal(tot, av, err)
	}
	if _, _, err := parseWindowsMem("{}"); err == nil {
		t.Fatal("want error")
	}
	if _, _, err := parseWindowsMem("x"); err == nil {
		t.Fatal("want error")
	}
}

func TestContainerHint(t *testing.T) {
	if !containerHint("0::/docker/abc") || containerHint("0::/user.slice") {
		t.Fatal("hint")
	}
}

func TestProbeLinuxAt(t *testing.T) {
	root := t.TempDir()
	w := func(p, s string) {
		p = filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(s), 0o644)
	}
	w("proc/meminfo", fixture(t, "meminfo.txt"))
	w("proc/loadavg", fixture(t, "loadavg.txt"))
	w("proc/1/cgroup", "0::/kubepods/x\n")
	w("cg/cpu.max", fixture(t, "cpu.max.limited"))
	w("cg/memory.max", fixture(t, "memory.max.limited"))
	var f hw.Facts
	probeLinuxAt(&f, root+"/proc", root+"/cg", root+"/none")
	if f.TotalRAMBytes == 0 || f.Load1 != 1.52 || f.CPUQuota != 1.5 || f.MemLimitBytes != 2<<30 || !f.Container || len(f.Notes) != 0 {
		t.Fatalf("%+v", f)
	}
	// cgroup v1 and missing files produce notes, not errors.
	root2 := t.TempDir()
	os.MkdirAll(filepath.Join(root2, "cg/cpu"), 0o755)
	os.MkdirAll(filepath.Join(root2, "cg/memory"), 0o755)
	os.WriteFile(filepath.Join(root2, "cg/cpu/cpu.cfs_quota_us"), []byte("200000\n"), 0o644)
	os.WriteFile(filepath.Join(root2, "cg/cpu/cpu.cfs_period_us"), []byte("100000\n"), 0o644)
	os.WriteFile(filepath.Join(root2, "cg/memory/memory.limit_in_bytes"), []byte("9223372036854771712\n"), 0o644)
	var g hw.Facts
	probeLinuxAt(&g, root2+"/proc", root2+"/cg", root2+"/none")
	if g.CPUQuota != 2 || g.MemLimitBytes != 0 || len(g.Notes) != 2 || g.Container {
		t.Fatalf("%+v", g)
	}
}

func TestProbeHost(t *testing.T) {
	f, err := New().Probe(context.Background(), ".")
	if err != nil {
		t.Fatal(err)
	}
	if f.OS != runtime.GOOS || f.Arch != runtime.GOARCH || f.LogicalCPUs < 1 || f.GOMAXPROCS < 1 || f.GoVersion == "" {
		t.Fatalf("%+v", f)
	}
	if runtime.GOOS != "other" && f.TotalRAMBytes == 0 && f.AvailRAMBytes == 0 {
		t.Errorf("no RAM info: %+v", f)
	}
}
