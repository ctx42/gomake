// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/goldy"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/pathkit"

	"github.com/ctx42/gomake/internal/mkf"
)

func Test_NewTargets(t *testing.T) {
	// --- When ---
	tgs := NewTargets()

	// --- Then ---
	assert.NotNil(t, tgs.unique)
	assert.NotNil(t, tgs.list)
	assert.False(t, tgs.sorted)
	assert.Equal(t, 0, tgs.Len())
}

func Test_TargetsFromList(t *testing.T) {
	t.Run("add", func(t *testing.T) {
		// --- Given ---
		tgt0 := &mkf.Target{Name: "tgt0"}
		tgt1 := &mkf.Target{Name: "tgt1"}

		// --- When ---
		tgs, err := TargetsFromList(tgt0, tgt1)

		// --- Then ---
		assert.NoError(t, err)
		assert.Same(t, tgt0, tgs.Get("tgt0"))
		assert.Same(t, tgt1, tgs.Get("tgt1"))
		assert.Equal(t, 2, tgs.Len())
	})

	t.Run("duplicate error", func(t *testing.T) {
		// --- Given ---
		tgt0 := &mkf.Target{Name: "tgt0"}
		tgt1 := &mkf.Target{Name: "tgt0"}

		// --- When ---
		tgs, err := TargetsFromList(tgt0, tgt1)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.Nil(t, tgs)
	})
}

func Test_TargetsFromSpecs(t *testing.T) {
	t.Run("as regular targets", func(t *testing.T) {
		// --- Given ---
		impSpecs := []string{
			"github.com/ctx42/gomake/testdata/imports/pkg0",
			"github.com/ctx42/gomake/testdata/imports/pkg1",
			"github.com/ctx42/gomake/testdata/imports/pkg7",
		}

		rng := ring.New()

		// --- When ---
		tgs, err := TargetsFromSpecs(rng, impSpecs)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			"pkg0",
			"pkg1",
			"print",
		}
		assert.Equal(t, want, tgs.Names())
	})

	t.Run("as built-in targets", func(t *testing.T) {
		// --- Given ---
		impSpecs := []string{
			"github.com/ctx42/gomake/testdata/imports/pkg0",
			"github.com/ctx42/gomake/testdata/imports/pkg1",
			"github.com/ctx42/gomake/testdata/imports/pkg7",
		}

		rng := ring.New()

		// --- When ---
		tgs, err := TargetsFromSpecs(rng, impSpecs, BuiltInCB)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			":pkg0",
			":pkg1",
			":print",
		}
		assert.Equal(t, want, tgs.Names())
	})

	t.Run("build environment applied", func(t *testing.T) {
		// --- Given ---
		impSpecs := []string{
			"github.com/ctx42/gomake/testdata/imports/pkg0",
			"github.com/ctx42/gomake/testdata/imports/pkg1",
			"github.com/ctx42/gomake/testdata/imports/pkg7",
		}

		rng := ring.New()
		rng.EnvSet("GOOS", "windows")

		// --- When ---
		tgs, err := TargetsFromSpecs(rng, impSpecs)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			"basic-win",
			"pkg0",
			"pkg1",
			"print",
		}
		assert.Equal(t, want, tgs.Names())
	})

	t.Run("duplicated target error", func(t *testing.T) {
		// --- Given ---
		// Both packages expose hello/bye; the tagged one needs the gomake
		// build tag so go list includes its files.
		impSpecs := []string{
			"github.com/ctx42/gomake/testdata/projects/simple_tagged/project",
			"github.com/ctx42/gomake/testdata/projects/simple_untagged/project",
		}

		rng := SetBuildTag(ring.New())

		// --- When ---
		tgs, err := TargetsFromSpecs(rng, impSpecs)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.Nil(t, tgs)
	})

	t.Run("invalid import", func(t *testing.T) {
		// --- Given ---
		impSpecs := []string{
			"github.com/ctx42/gomake/testdata/projects/empty",
		}

		rng := ring.New()

		// --- When ---
		tgs, err := TargetsFromSpecs(rng, impSpecs, BuiltInCB)

		// --- Then ---
		assert.ErrorIs(t, ErrGoList, err)
		assert.Nil(t, tgs)
	})

	t.Run("not existing import spec", func(t *testing.T) {
		// --- When ---
		impSpecs := []string{
			"github.com/ctx42/gomake/pkg/not_existing",
		}

		rng := ring.New()

		// --- When ---
		tgs, err := TargetsFromSpecs(rng, impSpecs)

		// --- Then ---
		assert.ErrorIs(t, ErrGoList, err)
		assert.Nil(t, tgs)
	})

	t.Run("error creating makefile instance", func(t *testing.T) {
		// --- When ---
		rng := ring.New()
		impSpecs := []string{
			"testdata/not_existing",
		}

		// --- When ---
		tgs, err := TargetsFromSpecs(rng, impSpecs)

		// --- Then ---
		assert.ErrorIs(t, ErrGoList, err)
		assert.Nil(t, tgs)
	})
}

func Test_TargetsFromImports(t *testing.T) {
	t.Run("namespace prefixes root targets", func(t *testing.T) {
		// --- Given ---
		imports := []Import{
			{
				Path:      "github.com/ctx42/gomake/testdata/imports/pkg0",
				Namespace: "myns",
			},
		}

		rng := ring.New()

		// --- When ---
		tgs, err := TargetsFromImports(rng, "", imports)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"myns:pkg0"}, tgs.Names())
	})

	t.Run("namespace prefixes type-based targets", func(t *testing.T) {
		// --- Given ---
		imports := []Import{
			{
				Path:      "github.com/ctx42/gomake/testdata/imports/pkg4",
				Namespace: "myns",
			},
		}

		rng := ring.New()

		// --- When ---
		tgs, err := TargetsFromImports(rng, "", imports)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"myns:ns:m0", "myns:ns:m1"}, tgs.Names())
	})

	t.Run("empty namespace leaves targets unprefixed", func(t *testing.T) {
		// --- Given ---
		imports := []Import{
			{Path: "github.com/ctx42/gomake/testdata/imports/pkg0"},
		}

		rng := ring.New()

		// --- When ---
		tgs, err := TargetsFromImports(rng, "", imports)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"pkg0"}, tgs.Names())
	})

	t.Run("namespace applied to built-in targets", func(t *testing.T) {
		// --- Given ---
		imports := []Import{
			{
				Path:      "github.com/ctx42/gomake/testdata/imports/pkg0",
				Namespace: "myns",
			},
		}

		rng := ring.New()

		// --- When ---
		tgs, err := TargetsFromImports(rng, "", imports, BuiltInCB)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{":myns:pkg0"}, tgs.Names())
	})
}

func Test_Targets_Add(t *testing.T) {
	t.Run("add not existing", func(t *testing.T) {
		// --- Given ---
		tgt0 := &mkf.Target{Name: "tgt0"}
		tgs := NewTargets()

		// --- When ---
		err := tgs.Add(tgt0)

		// --- Then ---
		assert.NoError(t, err)
		assert.Same(t, tgt0, tgs.Get("tgt0"))
		assert.Equal(t, 1, tgs.Len())
	})

	t.Run("add existing", func(t *testing.T) {
		// --- Given ---
		tgt0 := &mkf.Target{Name: "tgt0"}
		tgt1 := &mkf.Target{Name: "tgt0"}

		tgs := NewTargets()
		assert.NoError(t, tgs.Add(tgt0))

		// --- When ---
		err := tgs.Add(tgt1)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.Equal(t, 1, tgs.Len())
	})

	t.Run("add nothing", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()
		assert.NoError(t, tgs.Add(&mkf.Target{Name: "tgt0"}))

		// --- When ---
		err := tgs.Add()

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 1, tgs.Len())
	})
}

func Test_Targets_Has(t *testing.T) {
	t.Run("has", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()
		tgt0 := &mkf.Target{Name: "tgt0"}
		tgt1 := &mkf.Target{Name: "tgt1"}
		assert.NoError(t, tgs.Add(tgt0, tgt1))

		// --- When ---
		have := tgs.Has("tgt0")

		// --- Then ---
		assert.True(t, have)
	})

	t.Run("doesnt has", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()
		tgt0 := &mkf.Target{Name: "tgt0"}
		tgt1 := &mkf.Target{Name: "tgt1"}
		assert.NoError(t, tgs.Add(tgt0, tgt1))

		// --- When ---
		have := tgs.Has("tgt3")

		// --- Then ---
		assert.False(t, have)
	})

	t.Run("empty collection", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()

		// --- When ---
		have := tgs.Has("tgt3")

		// --- Then ---
		assert.False(t, have)
	})
}

func Test_Targets_addFunc(t *testing.T) {
	relPath := "testdata/projects/showcase_targets/project"
	absPath := modkit.Path(relPath)

	t.Run("add not existing", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		tgs := NewTargets()

		// --- When ---
		err := tgs.addFunc(tst.pkg, tst.Funcs()...)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			"basic",
			"basic-args",
			"basic-env",
			"long",
			"panic-string",
			"print-args",
			"say-hello",
		}
		assert.Equal(t, want, tgs.Names())
	})

	t.Run("add existing", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)
		assert.NoError(t, tgs.addFunc(tst.pkg, tst.Funcs()...))

		// --- When ---
		err := tgs.addFunc(tst.pkg, tst.Funcs()...)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
		assert.Len(t, 7, tgs.List())
	})
}

func Test_Targets_addType(t *testing.T) {
	t.Run("add targets across files", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)
		tgs := NewTargets()

		// --- When ---
		err := tgs.addType(tst.pkg, tst.Types()...)

		// --- Then ---
		assert.NoError(t, err)

		// makefile_ns_0.go
		assert.NotNil(t, tgs.Get("ns0:hello"))
		assert.NotNil(t, tgs.Get("ns0:ns1:hello"))
		assert.NotNil(t, tgs.Get("kebab-case:hello-world"))
		assert.NotNil(t, tgs.Get("ns0:ns1:ns2:hello-hello"))

		// makefile_ns_1.go
		assert.NotNil(t, tgs.Get("ns0:ns3:hello"))
		assert.NotNil(t, tgs.Get("ns0:ns3:ns4:hello"))

		// makefile_ns_with_default.go
		assert.NotNil(t, tgs.Get("ns-def"))
	})

	t.Run("file with incomplete namespaces", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)
		tgs := NewTargets()

		// --- When ---
		err := tgs.addType(tst.pkg, tst.Types()...)

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, tgs.Get("not0"))
		assert.Nil(t, tgs.Get("not1"))
		assert.Nil(t, tgs.Get("not2"))
		assert.Nil(t, tgs.Get("not3"))
		assert.Nil(t, tgs.Get("not4"))
		assert.Nil(t, tgs.Get("not5"))
		assert.Nil(t, tgs.Get("not6"))
	})

	t.Run("add existing", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		tgs := NewTargets()
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)
		assert.NoError(t, tgs.addType(tst.pkg, tst.Types()...))

		// --- When ---
		err := tgs.addType(tst.pkg, tst.Types()...)

		// --- Then ---
		assert.ErrorIs(t, ErrDupTarget, err)
	})

	t.Run("add namespace with default", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		tgs := NewTargets()
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		// --- When ---
		err := tgs.addType(tst.pkg, tst.Types()...)

		// --- Then ---
		assert.NoError(t, err)
		assert.NotNil(t, tgs.Get("ns-def:hello"))
		assert.NotNil(t, tgs.Get("ns-def:world"))
		assert.NotNil(t, tgs.Get("ns-def"))
	})
}

func Test_Targets_withReceiver(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()
		tgt0 := &mkf.Target{Name: "b", Receiver: "R0"}
		tgt1 := &mkf.Target{Name: "a", Receiver: "R1"}
		assert.NoError(t, tgs.Add(tgt0, tgt1))

		// --- When ---
		have := tgs.withReceiver("R0")

		// --- Then ---
		assert.Same(t, tgt0, have)
	})

	t.Run("not found", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()
		tgt0 := &mkf.Target{Name: "b", Receiver: "R0"}
		tgt1 := &mkf.Target{Name: "a", Receiver: "R1"}
		assert.NoError(t, tgs.Add(tgt0, tgt1))

		// --- When ---
		have := tgs.withReceiver("R2")

		// --- Then ---
		assert.Nil(t, have)
	})

	t.Run("empty", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()

		// --- When ---
		have := tgs.withReceiver("R2")

		// --- Then ---
		assert.Nil(t, have)
	})
}

func Test_Targets_Sort(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()

		// --- When ---
		tgs.Sort()

		// --- Then ---
		assert.Equal(t, 0, tgs.Len())
	})

	t.Run("sorts", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()
		tgt1 := &mkf.Target{Name: "b"}
		tgt0 := &mkf.Target{Name: "a"}
		assert.NoError(t, tgs.Add(tgt0, tgt1))

		// --- When ---
		tgs.Sort()

		// --- Then ---
		assert.True(t, tgs.sorted)

		var haveNames []string
		for _, info := range tgs.List() {
			haveNames = append(haveNames, info.Name)
		}
		assert.Equal(t, []string{"a", "b"}, haveNames)
	})
}

func Test_Targets_Get(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()

		// --- When ---
		have := tgs.Get("abc")

		// --- Then ---
		assert.Nil(t, have)
	})

	t.Run("get specific", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()
		tgt0 := &mkf.Target{Name: "a"}
		tgt1 := &mkf.Target{Name: "b"}
		assert.NoError(t, tgs.Add(tgt0, tgt1))

		// --- When ---
		have := tgs.Get("b")

		// --- Then ---
		assert.Same(t, tgt1, have)
	})
}

func Test_Targets_List(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()
		assert.NoError(t, tgs.Add(&mkf.Target{Name: "b"}))
		assert.NoError(t, tgs.Add(&mkf.Target{Name: "a"}))

		// --- When ---
		have := tgs.List()

		// --- Then ---
		var haveNames []string
		for _, info := range have {
			haveNames = append(haveNames, info.Name)
		}
		assert.Equal(t, []string{"a", "b"}, haveNames)
	})

	t.Run("empty", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()

		// --- When ---
		have := tgs.List()

		// --- Then ---
		assert.Len(t, 0, have)
		assert.NotNil(t, have)
	})

	t.Run("add makes it sort again", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()
		assert.NoError(t, tgs.Add(&mkf.Target{Name: "c"}))
		assert.NoError(t, tgs.Add(&mkf.Target{Name: "b"}))
		tgs.Sort()
		assert.NoError(t, tgs.Add(&mkf.Target{Name: "a"}))

		// --- When ---
		have := tgs.List()

		// --- Then ---
		var haveNames []string
		for _, info := range have {
			haveNames = append(haveNames, info.Name)
		}
		assert.Equal(t, []string{"a", "b", "c"}, haveNames)
	})
}

func Test_Targets_Names(t *testing.T) {
	t.Run("list is sorted", func(t *testing.T) {
		// --- Given ---
		tgt0 := &mkf.Target{Name: "tgt0"}
		tgt1 := &mkf.Target{Name: "tgt1"}
		tgt2 := &mkf.Target{Name: "tgt2"}
		tgs := NewTargets()
		assert.NoError(t, tgs.Add(tgt2, tgt0, tgt1))

		// --- When ---
		have := tgs.Names()

		// --- Then ---
		assert.Equal(t, []string{"tgt0", "tgt1", "tgt2"}, have)
	})

	t.Run("empty list", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()

		// --- When ---
		have := tgs.Names()

		// --- Then ---
		assert.Equal(t, []string{}, have)
		assert.NotNil(t, have)
	})
}

func Test_Targets_MarkDefault(t *testing.T) {
	t.Run("empty collection", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()

		// --- When ---
		have := tgs.MarkDefault("tgt0")

		// --- Then ---
		assert.Equal(t, "", have)
	})

	t.Run("mark proper target", func(t *testing.T) {
		// --- Given ---
		tgt0 := &mkf.Target{Name: "tgt0", DefRef: "Tgt0"}
		tgt1 := &mkf.Target{Name: "tgt1", DefRef: "Tgt1"}

		tgs := NewTargets()
		assert.NoError(t, tgs.Add(tgt0, tgt1))

		// --- When ---
		have := tgs.MarkDefault("Tgt0")

		// --- Then ---
		assert.Equal(t, "tgt0", have)
		assert.True(t, tgs.Get("tgt0").Default)
		assert.False(t, tgs.Get("tgt1").Default)
	})

	t.Run("change default target", func(t *testing.T) {
		// --- Given ---
		tgt0 := &mkf.Target{Name: "tgt0", DefRef: "Tgt0", Default: true}
		tgt1 := &mkf.Target{Name: "tgt1", DefRef: "Tgt1", Default: false}

		tgs := NewTargets()
		assert.NoError(t, tgs.Add(tgt0, tgt1))

		// --- When ---
		have := tgs.MarkDefault("Tgt1")

		// --- Then ---
		assert.Equal(t, "tgt1", have)
		assert.False(t, tgs.Get("tgt0").Default)
		assert.True(t, tgs.Get("tgt1").Default)
	})
}

func Test_BuiltInCB(t *testing.T) {
	// --- Given ---
	tgt0 := &mkf.Target{Name: "tgt0"}
	tgt1 := &mkf.Target{Name: "tgt1"}
	tgt2 := &mkf.Target{Name: "tgt2"}
	tgs := must.Value(TargetsFromList(tgt0, tgt1, tgt2))

	// --- When ---
	tgs.Map(BuiltInCB)

	// --- Then ---
	want := []string{
		":tgt0",
		":tgt1",
		":tgt2",
	}
	assert.Equal(t, want, tgs.Names())
	assert.HasKey(t, ":tgt0", tgs.unique)
	assert.HasKey(t, ":tgt1", tgs.unique)
	assert.HasKey(t, ":tgt2", tgs.unique)
}

func Test_Targets_Map(t *testing.T) {
	// --- Given ---
	tgt0 := &mkf.Target{Name: "tgt0"}
	tgt1 := &mkf.Target{Name: "tgt1"}

	tgs := NewTargets()
	assert.NoError(t, tgs.Add(tgt0, tgt1))

	// --- When ---
	fn := func(tgs *Targets, tgt *mkf.Target) {
		delete(tgs.unique, tgt.Name)
		tgt.Name = ":" + tgt.Name
		tgt.Doc = "abc"
		tgs.unique[tgt.Name] = struct{}{}
	}
	tgs.Map(fn)

	// --- Then ---
	assert.Equal(t, "abc", tgs.Get(":tgt0").Doc)
	assert.Equal(t, "abc", tgs.Get(":tgt1").Doc)
	assert.Equal(t, 2, tgs.Len())
}

func Test_Targets_GoImports(t *testing.T) {
	t.Run("no imports", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()

		// --- When ---
		have := tgs.GoImports()

		// --- Then ---
		assert.Equal(t, "", have)
	})

	t.Run("imports", func(t *testing.T) {
		// --- Given ---
		tgt0 := &mkf.Target{
			Name:    "a",
			PkgName: "pkg1",
			ImpSpec: "git.com/prj/pkg1",
		}
		tgt1 := &mkf.Target{
			Name:    "b",
			PkgName: "pkg2",
			ImpSpec: "git.com/prj/pkg2",
		}
		tgt2 := &mkf.Target{
			Name:    "c",
			PkgName: "pkg3",
			ImpSpec: "git.com/prj/pkg3",
		}
		tgt3 := &mkf.Target{Name: "e", PkgName: "main", ImpSpec: ""}
		tgs := NewTargets()
		assert.NoError(t, tgs.Add(tgt0, tgt1, tgt2, tgt3))

		// --- When ---
		have := tgs.GoImports()

		// --- Then ---
		want := "\n" +
			"\"git.com/prj/pkg1\"\n" +
			"\"git.com/prj/pkg2\"\n" +
			"\"git.com/prj/pkg3\"\n"
		assert.Equal(t, want, have)
	})

	t.Run("duplicates are removed", func(t *testing.T) {
		// --- Given ---
		tgt0 := &mkf.Target{
			Name:    "a",
			PkgName: "pkg1",
			ImpSpec: "git.com/prj/pkg1",
		}
		tgt1 := &mkf.Target{
			Name:    "b",
			PkgName: "pkg1",
			ImpSpec: "git.com/prj/pkg1",
		}
		tgt2 := &mkf.Target{
			Name:    "c",
			PkgName: "pkg3",
			ImpSpec: "git.com/prj/pkg3",
		}
		tgs := NewTargets()
		assert.NoError(t, tgs.Add(tgt0, tgt1, tgt2))

		// --- When ---
		have := tgs.GoImports()

		// --- Then ---
		want := "\n" +
			"\"git.com/prj/pkg1\"\n" +
			"\"git.com/prj/pkg3\"\n"
		assert.Equal(t, want, have)
	})

	t.Run("same package name gets aliases", func(t *testing.T) {
		// --- Given ---
		tgt0 := &mkf.Target{
			Name:    "a",
			PkgName: "git",
			ImpSpec: "example.com/one/git",
		}
		tgt1 := &mkf.Target{
			Name:    "b",
			PkgName: "git",
			ImpSpec: "example.com/two/git",
		}
		tgs := NewTargets()
		assert.NoError(t, tgs.Add(tgt0, tgt1))

		// --- When ---
		have := tgs.GoImports()

		// --- Then ---
		// Sorted by import path; first keeps default name, second aliases.
		want := "\n" +
			"\"example.com/one/git\"\n" +
			"git2 \"example.com/two/git\"\n"
		assert.Equal(t, want, have)
	})

	t.Run("imports from showcase example", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		relPath := "testdata/projects/showcase_imports/project"
		absPath := modkit.Path(relPath)

		pmf := must.Value(NewMakefile(rng, absPath))

		// --- When ---
		have := pmf.Targets.GoImports()

		// --- Then ---
		want := "\n" +
			`"github.com/ctx42/gomake/testdata/imports/pkg0"` + "\n" +
			`"github.com/ctx42/gomake/testdata/imports/pkg1"` + "\n" +
			`"github.com/ctx42/gomake/testdata/imports/pkg2"` + "\n" +
			`"github.com/ctx42/gomake/testdata/imports/pkg9"` + "\n"
		assert.Equal(t, want, have)
	})

	t.Run("imports from showcase example GOOS windows", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("GOOS", "windows")
		relPath := "testdata/projects/showcase_imports/project"
		absPath := modkit.Path(relPath)

		pmf := must.Value(NewMakefile(rng, absPath))

		// --- When ---
		have := pmf.Targets.GoImports()

		// --- Then ---
		want := "\n" +
			`"github.com/ctx42/gomake/testdata/imports/pkg0"` + "\n" +
			`"github.com/ctx42/gomake/testdata/imports/pkg1"` + "\n" +
			`"github.com/ctx42/gomake/testdata/imports/pkg2"` + "\n" +
			`"github.com/ctx42/gomake/testdata/imports/pkg3"` + "\n" +
			`"github.com/ctx42/gomake/testdata/imports/pkg9"` + "\n"
		assert.Equal(t, want, have)
	})
}

func Test_Targets_GoCode(t *testing.T) {
	t.Run("target from main package", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		gfp := pathkit.AbsPath(t, "testdata/target_from_main.gld")
		gfd := map[string]any{"prj_root": modkit.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		tgs := NewTargets()
		assert.NoError(t, tgs.addFunc(tst.pkg, tst.Func("Basic")))

		// --- When ---
		have := tgs.GoCode(false)

		// --- Then ---
		assert.Equal(t, gld.String(), have)
	})

	t.Run("target from imported package", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/imports/pkg1"
		absPath := modkit.Path(relPath)
		gfp := pathkit.AbsPath(t, "testdata/target_from_imported.gld")
		gfd := map[string]any{"prj_root": modkit.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		tgs := NewTargets()
		assert.NoError(t, tgs.addFunc(tst.pkg, tst.Func("Pkg1")))

		// --- When ---
		have := tgs.GoCode(false)

		// --- Then ---
		assert.Equal(t, gld.String(), have)
	})

	t.Run("namespaced target from main package", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		gfp := pathkit.AbsPath(t, "testdata/target_from_main_ns.gld")
		gfd := map[string]any{"prj_root": modkit.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		tgs := NewTargets()
		assert.NoError(t, tgs.addType(tst.pkg, tst.Type("NS0")))

		// --- When ---
		have := tgs.GoCode(false)

		// --- Then ---
		assert.Equal(t, gld.String(), have)
	})

	t.Run("namespaced target from imported package", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/imports/pkg4"
		absPath := modkit.Path(relPath)
		gfp := pathkit.AbsPath(t, "testdata/target_from_imported_ns.gld")
		gfd := map[string]any{"prj_root": modkit.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		tgs := NewTargets()
		assert.NoError(t, tgs.addType(tst.pkg, tst.Type("NS")))

		// --- When ---
		have := tgs.GoCode(false)

		// --- Then ---
		assert.Equal(t, gld.String(), have)
	})

	t.Run("targets with core qualified identifier", func(t *testing.T) {
		// --- Given ---
		relPath := "testdata/projects/showcase_targets/project"
		absPath := modkit.Path(relPath)
		gfp := pathkit.AbsPath(t, "testdata/target_qualified_code.gld")
		gfd := map[string]any{"prj_root": modkit.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))
		rng := ring.New()
		tst := NewTestHelper(t, rng, absPath)

		tgs := NewTargets()
		assert.NoError(t, tgs.addFunc(tst.pkg, tst.Func("Basic")))

		// --- When ---
		have := tgs.GoCode(true)

		// --- Then ---
		assert.Equal(t, gld.String(), have)
	})

	t.Run("no targets", func(t *testing.T) {
		// --- Given ---
		tgs := NewTargets()

		// --- When ---
		have := tgs.GoCode(false)

		// --- Then ---
		want := "targets := make([]*Target, 0)"
		assert.Equal(t, want, have)
	})

	t.Run("first target hidden", func(t *testing.T) {
		// --- Given ---
		gfp := pathkit.AbsPath(t, "testdata/targets_first_hidden.gld")
		gfd := map[string]any{"prj_root": modkit.Root()}
		gld := goldy.Open(t, gfp, goldy.WithData(gfd))

		tgt0 := &mkf.Target{
			Name:    "a",
			PkgName: "pkg1",
			CodeRef: "pkg1.Target",
			ImpSpec: "git.com/prj/pkg1",
			Hidden:  true,
		}
		tgt1 := &mkf.Target{
			Name:    "b",
			PkgName: "pkg2",
			CodeRef: "pkg2.Target",
			ImpSpec: "git.com/prj/pkg2",
			Hidden:  false,
		}
		tgs := NewTargets()
		assert.NoError(t, tgs.Add(tgt0, tgt1))

		// --- When ---
		have := tgs.GoCode(false)

		// --- Then ---
		assert.Equal(t, gld.String(), have)
	})
}
