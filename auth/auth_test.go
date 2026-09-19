package auth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveProfileBlockRemovesOnlyTargetProfile(t *testing.T) {
	content := `[profile infraresc]
region = ap-south-1

[profile other]
region = us-east-1
`

	got := removeProfileBlock(content, "infraresc")

	if strings.Contains(got, "[profile infraresc]") {
		t.Fatalf("target profile was not removed: %q", got)
	}
	if !strings.Contains(got, "[profile other]") {
		t.Fatalf("unrelated profile was removed: %q", got)
	}
}

func TestWriteAWSProfileCreatesAndReplacesProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := WriteAWSProfile(AWSProfile{Name: "infraresc", Region: "ap-south-1"}); err != nil {
		t.Fatalf("WriteAWSProfile returned error: %v", err)
	}

	path := filepath.Join(home, ".aws", "config")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read generated config: %v", err)
	}

	if got := string(data); !strings.Contains(got, "[profile infraresc]\nregion = ap-south-1") {
		t.Fatalf("generated config missing profile: %q", got)
	}

	if err := WriteAWSProfile(AWSProfile{Name: "infraresc", Region: "us-east-1"}); err != nil {
		t.Fatalf("second WriteAWSProfile returned error: %v", err)
	}

	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to reread generated config: %v", err)
	}

	got := string(data)
	if strings.Count(got, "[profile infraresc]") != 1 {
		t.Fatalf("expected exactly one profile block, got: %q", got)
	}
	if !strings.Contains(got, "region = us-east-1") {
		t.Fatalf("profile was not updated: %q", got)
	}
	if strings.Contains(got, "region = ap-south-1") {
		t.Fatalf("old region remains after replacement: %q", got)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat AWS config: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("AWS config permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestWriteAWSProfileRejectsMissingRequiredFields(t *testing.T) {
	for _, profile := range []AWSProfile{
		{Name: "", Region: "ap-south-1"},
		{Name: "infraresc", Region: ""},
		{Name: "   ", Region: "ap-south-1"},
		{Name: "infraresc", Region: "   "},
	} {
		if err := WriteAWSProfile(profile); err == nil {
			t.Fatalf("WriteAWSProfile(%#v) expected an error", profile)
		}
	}
}

func TestManagerDelegatesToProvider(t *testing.T) {
	provider := &fakeProvider{identity: &Identity{Provider: "AWS", Authenticated: true}}
	manager := NewManager(provider)

	identity, err := manager.Status(context.Background(), LoginOptions{Profile: "test"})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if identity != provider.identity {
		t.Fatalf("Status returned a different identity")
	}
	if provider.lastProfile != "test" {
		t.Fatalf("Status profile = %q, want test", provider.lastProfile)
	}
}

type fakeProvider struct {
	identity    *Identity
	lastProfile string
}

func (f *fakeProvider) Login(_ context.Context, opts LoginOptions) (*Identity, error) {
	f.lastProfile = opts.Profile
	return f.identity, nil
}

func (f *fakeProvider) Logout(_ context.Context, opts LoginOptions) error {
	f.lastProfile = opts.Profile
	return nil
}

func (f *fakeProvider) Status(_ context.Context, opts LoginOptions) (*Identity, error) {
	f.lastProfile = opts.Profile
	return f.identity, nil
}
