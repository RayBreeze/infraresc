package cli

import "testing"

func TestRootCommandRegistersImplementedCommandGroups(t *testing.T) {
	for _, name := range []string{"auth", "scan", "snapshot", "diff", "recover", "media", "verify"} {
		if rootCmd.Commands() == nil {
			t.Fatal("root command has no registered commands")
		}

		cmd, _, err := rootCmd.Find([]string{name})
		if err != nil {
			t.Fatalf("find %q: %v", name, err)
		}
		if cmd == nil || cmd.Name() != name {
			t.Fatalf("expected root command %q to be registered", name)
		}
	}
}

func TestScanAndSnapshotExposeProfileAndOutputFlags(t *testing.T) {
	profile := scanCmd.Flags().Lookup("profile")
	if profile == nil || profile.Shorthand != "p" {
		t.Fatal("scan should expose --profile/-p")
	}

	snapshotProfileFlag := snapshotCmd.Flags().Lookup("profile")
	if snapshotProfileFlag == nil || snapshotProfileFlag.Shorthand != "p" {
		t.Fatal("snapshot should expose --profile/-p")
	}

	output := snapshotCmd.Flags().Lookup("output")
	if output == nil || output.Shorthand != "o" {
		t.Fatal("snapshot should expose --output/-o")
	}
}
