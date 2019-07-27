package main

import (
	"go/build"
	"testing"
)

func Test_mmcImporter(t *testing.T) {
	// this test is meant to be a manual debugging aid, comment
	// this out when debugging.
	t.Skip()

	imp := &mmcImporter{
		Ctx: build.Default,
	}

	const packagePath = "github.com/minio/minio-go"

	pkg, err := imp.Import(packagePath)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Package: ", pkg)
}

func Test_removeMajorVersionFromPath(t *testing.T) {
	tests := []struct {
		In               string
		Want             string
		WantRemovedCount int
	}{
		{
			"example.com/org/library/v6/pkg",
			"example.com/org/library/pkg",
			3,
		},
		{
			"example.com/org/library/v999/pkg",
			"example.com/org/library/pkg",
			5,
		},
		{
			"example.com/org/library/pkg",
			"example.com/org/library/pkg",
			0,
		},
		// with quotes
		{
			`"example.com/org/library/v3333/subpkg"`,
			`"example.com/org/library/subpkg"`,
			6,
		},
		{
			`"example.com/org/library/v33"`,
			`"example.com/org/library"`,
			4,
		},
		{
			`"example.com/org/library/pkg"`,
			`"example.com/org/library/pkg"`,
			0,
		},
		{
			`"example.com/org/gomobile-v2mod/v2"`,
			`"example.com/org/gomobile-v2mod"`,
			3,
		},
	}

	for i, tst := range tests {
		r, n := removeMajorVersionFromPath(tst.In)
		if r != tst.Want {
			t.Error("Failed on", i, " wanted ", tst.Want, " got ", r)
		}

		if n != tst.WantRemovedCount {
			t.Error("Failed on removed count", i, " wanted ", tst.WantRemovedCount, " got ", n)
		}
	}
}

func Test_isVNPart(t *testing.T) {
	tests := []struct {
		In   string
		Want bool
	}{
		{"abc", false},
		{`/v1"`, false},
		{"/v44", false},
		{"v44", true},
		{"v0", true},
		{`v0"`, true},
		{`v1"`, true},
		{`v3333"`, true},
	}

	for i, tst := range tests {
		r := isVNPart(tst.In)
		if r != tst.Want {
			t.Error("failed on", i, "expected ", tst.Want, "got", r)
		}
	}
}
