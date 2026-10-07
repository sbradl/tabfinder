package finder

import (
	"fmt"
	"slices"
	"testing"

	"tabfinder/internal/tab"
)

func songs(artistsAndTitles ...string) []*tab.Song {
	var out []*tab.Song
	for i := 0; i+1 < len(artistsAndTitles); i += 2 {
		out = append(out, &tab.Song{Path: fmt.Sprint(i/2, ".gp5"), Artist: artistsAndTitles[i], Title: artistsAndTitles[i+1]})
	}
	return out
}

func order(l *Library) []string {
	var out []string
	for _, e := range l.Entries {
		out = append(out, e.Song.Path)
	}
	return out
}

func TestNewOrder(t *testing.T) {
	t.Run("artist then title, case-insensitive", func(t *testing.T) {
		l := New(songs("b", "x", "A", "z", "a", "Y", "B", "a", "äther", "m", "Zed", "q"))
		var got []string
		for _, e := range l.Entries {
			got = append(got, e.Song.Artist+"/"+e.Song.Title)
		}
		want := []string{"A/z", "a/Y", "B/a", "b/x", "Zed/q", "äther/m"}
		// "A/z" before "a/Y": same artist ignoring case, so by title: Y < z.
		want = []string{"a/Y", "A/z", "B/a", "b/x", "Zed/q", "äther/m"}
		if !slices.Equal(got, want) {
			t.Errorf("order = %q, want %q", got, want)
		}
	})
	t.Run("stable for equal keys", func(t *testing.T) {
		in := songs("A", "Same", "a", "same", "A", "SAME")
		in[0].Path, in[1].Path, in[2].Path = "first", "second", "third"
		l := New(in)
		if got := order(l); !slices.Equal(got, []string{"first", "second", "third"}) {
			t.Errorf("order = %q", got)
		}
	})
	t.Run("input slice is left alone", func(t *testing.T) {
		in := songs("b", "x", "a", "y")
		New(in)
		if in[0].Artist != "b" {
			t.Error("input reordered")
		}
	})
	t.Run("empty and nil", func(t *testing.T) {
		for _, in := range [][]*tab.Song{nil, {}} {
			l := New(in)
			if l.Entries == nil || len(l.Entries) != 0 || l.Artists == nil || len(l.Artists) != 0 {
				t.Errorf("New(%v) = %#v", in, l)
			}
			// All queries on an empty library give empty, non-nil lists.
			r := l.Search(Query{Artist: "x", Tuning: "y", Name: "z"})
			if r.Matches == nil || r.Tunings == nil || l.SuggestArtists("") == nil {
				t.Errorf("nil list from empty library: %+v", r)
			}
		}
	})
}

func TestNewEntry(t *testing.T) {
	t.Run("duplicates collapsed, strings kept apart, most strings first", func(t *testing.T) {
		s := &tab.Song{Tracks: []tab.Track{dropC4, dropC6, dropC6, dropC4, eStd6, bStd7}}
		e := newEntry(s)
		want := []Tuning{{7, "B Standard", "B E A D G B E"}, {6, "Drop C", "C G C F A D"}, {6, "E Standard", "E A D G B E"}, {4, "Drop C", "C G C F"}}
		if !slices.Equal(e.Tunings, want) {
			t.Errorf("tunings = %v\nwant      %v", e.Tunings, want)
		}
	})
	t.Run("same name different string counts", func(t *testing.T) {
		e := newEntry(&tab.Song{Tracks: []tab.Track{dropC6, dropC4}})
		if len(e.Tunings) != 2 || e.Tunings[0].Strings != 6 || e.Tunings[1].Strings != 4 {
			t.Errorf("tunings = %v", e.Tunings)
		}
	})
	t.Run("equal string counts keep track order", func(t *testing.T) {
		e := newEntry(&tab.Song{Tracks: []tab.Track{eStd6, dropC6}})
		if e.Tunings[0].Name != "E Standard" || e.Tunings[1].Name != "Drop C" {
			t.Errorf("tunings = %v", e.Tunings)
		}
	})
	t.Run("drums and untuned tracks skipped", func(t *testing.T) {
		e := newEntry(&tab.Song{Tracks: []tab.Track{{Name: "Drums", Drums: true}, {Name: "Vocals"}, eStd6}})
		if len(e.Tunings) != 1 {
			t.Errorf("tunings = %v", e.Tunings)
		}
	})
	t.Run("no tracks", func(t *testing.T) {
		if e := newEntry(&tab.Song{}); len(e.Tunings) != 0 {
			t.Errorf("entry = %+v", e)
		}
	})
	t.Run("custom tuning is labelled by its notes", func(t *testing.T) {
		e := newEntry(&tab.Song{Tracks: []tab.Track{{Pitches: []int{1, 2, 3, 4}}}})
		if want := (Tuning{4, "Custom", "C# D Eb E"}); len(e.Tunings) != 1 || e.Tunings[0] != want {
			t.Errorf("tunings = %v", e.Tunings)
		}
		if got := e.Tunings[0].Label(); got != "C# D Eb E" {
			t.Errorf("label = %q", got)
		}
	})
}

func TestTuningLabel(t *testing.T) {
	for _, tt := range []struct {
		t    Tuning
		want string
	}{
		{Tuning{6, "Drop C", "C G C F A D"}, "Drop C"},
		{Tuning{6, "Custom", "D G D G B D"}, "D G D G B D"},
		{Tuning{6, "Custom", ""}, "Custom"},
		{Tuning{6, "custom", "D G"}, "custom"}, // only the exact name "Custom"
		{Tuning{}, ""},
	} {
		if got := tt.t.Label(); got != tt.want {
			t.Errorf("%+v.Label() = %q, want %q", tt.t, got, tt.want)
		}
	}
}

func TestArtistsOf(t *testing.T) {
	t.Run("most common spelling wins", func(t *testing.T) {
		l := New(songs("Inkwell Flamingos", "a", "INKWELL FLAMINGOS", "b", "Inkwell Flamingos", "c", "inkwell flamingos", "d", "INKWELL FLAMINGOS", "e", "INKWELL FLAMINGOS", "f"))
		if want := []string{"INKWELL FLAMINGOS"}; !slices.Equal(l.Artists, want) {
			t.Errorf("artists = %q", l.Artists)
		}
	})
	t.Run("tie goes to the alphabetically first", func(t *testing.T) {
		l := New(songs("INKWELL FLAMINGOS", "a", "Inkwell Flamingos", "b"))
		if want := []string{"INKWELL FLAMINGOS"}; !slices.Equal(l.Artists, want) { // "I" < "n" in byte order
			t.Errorf("artists = %q", l.Artists)
		}
		l = New(songs("inkwell flamingos", "a", "Inkwell Flamingos", "b"))
		if want := []string{"Inkwell Flamingos"}; !slices.Equal(l.Artists, want) {
			t.Errorf("artists = %q", l.Artists)
		}
	})
	t.Run("blank skipped, spaces trimmed, A-Z ignoring case", func(t *testing.T) {
		l := New(songs("", "a", "   ", "b", "\t", "c", "  zed ", "d", "Zed", "e", "alpha", "f", "Beta", "g", "äther", "h"))
		if want := []string{"alpha", "Beta", "Zed", "äther"}; !slices.Equal(l.Artists, want) {
			t.Errorf("artists = %q, want %q", l.Artists, want)
		}
	})
	t.Run("trimmed spellings are counted together", func(t *testing.T) {
		l := New(songs(" X ", "a", "X", "b", "x", "c"))
		if want := []string{"X"}; !slices.Equal(l.Artists, want) {
			t.Errorf("artists = %q", l.Artists)
		}
	})
	t.Run("deterministic", func(t *testing.T) {
		in := songs("Aa", "1", "AA", "2", "aA", "3", "aa", "4")
		first := New(in).Artists
		for range 20 {
			if got := New(in).Artists; !slices.Equal(got, first) {
				t.Fatalf("artists changed: %q vs %q", got, first)
			}
		}
		if first[0] != "AA" {
			t.Errorf("tie of 4 = %q, want AA", first[0])
		}
	})
}

func TestQueryFilter(t *testing.T) {
	t.Run("trimmed and passed through", func(t *testing.T) {
		f, invalid := Query{Name: "  a b ", Artist: " c ", Tuning: " d e ", BPM: " 100 - 140 ", Strings: 7}.Filter()
		if invalid || f.Name != "a b" || f.Artist != "c" || f.Tuning != "d e" || f.Strings != 7 || f.BPM == nil || *f.BPM != (BPMRange{Min: 100, Max: 140}) {
			t.Errorf("filter = %+v, invalid %v", f, invalid)
		}
	})
	t.Run("blank BPM is no filter", func(t *testing.T) {
		for _, bpm := range []string{"", " ", "\t "} {
			f, invalid := Query{BPM: bpm}.Filter()
			if invalid || f.BPM != nil {
				t.Errorf("BPM %q: filter = %+v, invalid %v", bpm, f, invalid)
			}
		}
	})
	t.Run("invalid BPM is reported and ignored", func(t *testing.T) {
		for _, bpm := range []string{"fast", "-", "140-100", "1-2-3"} {
			f, invalid := Query{BPM: bpm, Name: "x"}.Filter()
			if !invalid || f.BPM != nil || f.Name != "x" {
				t.Errorf("BPM %q: filter = %+v, invalid %v", bpm, f, invalid)
			}
		}
	})
	t.Run("active", func(t *testing.T) {
		if (Query{}).Active() {
			t.Error("empty query is active")
		}
		for _, q := range []Query{{Name: "a"}, {Artist: "a"}, {Tuning: "a"}, {BPM: "a"}, {Strings: 6}} {
			if !q.Active() {
				t.Errorf("%+v is not active", q)
			}
		}
		// Active looks at the fields as typed: blanks count.
		if !(Query{Name: " "}).Active() {
			t.Error("Query{Name: \" \"} is not active")
		}
	})
}

func TestSearchTunings(t *testing.T) {
	// 6-string: Drop C x3, E Std x3, Custom x1; 7: B Std x2; 8: custom x1; 9: custom x1; 4: Drop C x2, E std x1; 5: B Std x1.
	nine := tab.Track{Pitches: []int{25, 30, 35, 40, 45, 50, 55, 59, 64}}
	eight := tab.Track{Pitches: []int{30, 35, 40, 45, 50, 55, 59, 64}}
	bass5 := tab.Track{Pitches: []int{23, 28, 33, 38, 43}}
	eStd4 := tab.Track{Pitches: []int{28, 33, 38, 43}}
	in := []*tab.Song{
		{Artist: "A", Title: "1", Tracks: []tab.Track{dropC6, dropC4}},
		{Artist: "A", Title: "2", Tracks: []tab.Track{dropC6, dropC4}},
		{Artist: "B", Title: "3", Tracks: []tab.Track{dropC6, eStd6, bass5}},
		{Artist: "B", Title: "4", Tracks: []tab.Track{eStd6, eStd4}},
		{Artist: "C", Title: "5", Tracks: []tab.Track{eStd6, custom}},
		{Artist: "C", Title: "6", Tracks: []tab.Track{bStd7}},
		{Artist: "C", Title: "7", Tracks: []tab.Track{bStd7, eight}},
		{Artist: "C", Title: "8", Tracks: []tab.Track{nine}},
	}
	l := New(in)
	want := []Tuning{
		{6, "Drop C", "C G C F A D"}, {6, "E Standard", "E A D G B E"}, {6, "Custom", "B E A D G B"},
		{7, "B Standard", "B E A D G B E"},
		{8, "Custom", "F# B E A D G B E"},
		{9, "Custom", "C# F# B E A D G B E"},
		{4, "Drop C", "C G C F"}, {4, "E Standard", "E A D G"},
		{5, "B Standard", "B E A D G"},
	}
	// Drop C and E Standard (6) both appear 3 times: ties by label, "Drop C" < "E Standard".
	got := l.Search(Query{}).Tunings
	if !slices.Equal(got, want) {
		t.Errorf("tunings = %v\nwant      %v", got, want)
	}

	t.Run("artist trims", func(t *testing.T) {
		got := l.Search(Query{Artist: "b"}).Tunings
		want := []Tuning{{6, "E Standard", "E A D G B E"}, {6, "Drop C", "C G C F A D"}, {4, "E Standard", "E A D G"}, {5, "B Standard", "B E A D G"}} // E Standard is on two of B's songs
		if !slices.Equal(got, want) {
			t.Errorf("tunings = %v\nwant      %v", got, want)
		}
	})
	t.Run("name and bpm trim", func(t *testing.T) {
		if got := l.Search(Query{Name: "6"}).Tunings; len(got) != 1 || got[0].Strings != 7 {
			t.Errorf("tunings = %v", got)
		}
	})
	t.Run("typed tuning and strings do not trim", func(t *testing.T) {
		for _, q := range []Query{{Tuning: "drop c"}, {Tuning: "zzz"}, {Strings: 4}, {Tuning: "drop c", Strings: 6}} {
			if got := l.Search(q).Tunings; !slices.Equal(got, want) {
				t.Errorf("Search(%+v).Tunings = %v", q, got)
			}
		}
	})
	t.Run("invalid BPM does not trim", func(t *testing.T) {
		if got := l.Search(Query{BPM: "fast"}).Tunings; !slices.Equal(got, want) {
			t.Errorf("tunings = %v", got)
		}
	})
	t.Run("no matches", func(t *testing.T) {
		r := l.Search(Query{Artist: "nobody"})
		if r.Tunings == nil || len(r.Tunings) != 0 || r.Matches == nil || len(r.Matches) != 0 {
			t.Errorf("result = %#v", r)
		}
	})
	t.Run("most common first within a group", func(t *testing.T) {
		r := New(songs("A", "1", "A", "2", "A", "3")) // no tunings at all
		if got := r.Search(Query{}).Tunings; len(got) != 0 {
			t.Errorf("tunings = %v", got)
		}
		in := []*tab.Song{{Tracks: []tab.Track{eStd6}}, {Tracks: []tab.Track{dropC6}}, {Tracks: []tab.Track{dropC6}}}
		got := New(in).Search(Query{}).Tunings
		if got[0].Name != "Drop C" || got[1].Name != "E Standard" {
			t.Errorf("tunings = %v", got)
		}
	})
}

func TestSuggestArtists(t *testing.T) {
	l := New(songs("Inkwell Flamingos", "a", "Flamingo Hour", "b", "Amber Marsh", "c", "Slam", "d", "Soilbed Quartet", "e", "Amphora", "f"))
	check := func(typed string, want ...string) {
		t.Helper()
		got := l.SuggestArtists(typed)
		if got == nil {
			t.Errorf("SuggestArtists(%q) = nil", typed)
		}
		if !slices.Equal(got, want) && !(len(got) == 0 && len(want) == 0) {
			t.Errorf("SuggestArtists(%q) = %q, want %q", typed, got, want)
		}
	}
	check("am", "Amber Marsh", "Amphora", "Flamingo Hour", "Inkwell Flamingos", "Slam")                  // prefix matches, then substring in A-Z order
	check("fl", "Flamingo Hour", "Inkwell Flamingos")                                                    // Flamingo Hour first (prefix)
	check("FLAMINGO", "Flamingo Hour", "Inkwell Flamingos")                                              // case-insensitive
	check("  flamingo  ", "Flamingo Hour", "Inkwell Flamingos")                                          // trimmed
	check("inkwell flamingos")                                                                           // exact match leaves nothing but...
	check("zzz")                                                                                         // no matches
	check("", "Amber Marsh", "Amphora", "Flamingo Hour", "Inkwell Flamingos", "Slam", "Soilbed Quartet") // all
	check("   ", "Amber Marsh", "Amphora", "Flamingo Hour", "Inkwell Flamingos", "Slam", "Soilbed Quartet")
	check("SLAM")        // exact, any case: excluded
	check("slam ")       // exact after trimming
	check("sla", "Slam") // not exact
	check("o", "Amphora", "Flamingo Hour", "Inkwell Flamingos", "Soilbed Quartet")

	t.Run("capped", func(t *testing.T) {
		var in []*tab.Song
		for i := range 200 {
			in = append(in, &tab.Song{Artist: fmt.Sprintf("Band %03d", i)})
		}
		l := New(in)
		if got := l.SuggestArtists(""); len(got) != MaxSuggestions || MaxSuggestions != 60 {
			t.Errorf("len = %d, max %d", len(got), MaxSuggestions)
		}
		if got := l.SuggestArtists("band"); len(got) != 60 || got[0] != "Band 000" {
			t.Errorf("len = %d, first %q", len(got), got[0])
		}
		// Prefix matches fill the cap before substring matches get in.
		in = append(in, &tab.Song{Artist: "A band"})
		if got := New(in).SuggestArtists("band"); slices.Contains(got, "A band") {
			t.Error("substring match displaced a prefix match")
		}
	})
}

func TestSuggestTunings(t *testing.T) {
	ts := []Tuning{
		{6, "Drop C", "C G C F A D"}, {6, "Custom", "B E A D G B"}, {6, "E Standard", "E A D G B E"},
		{7, "B Standard", "B E A D G B E"}, {4, "Drop C", "C G C F"},
	}
	check := func(typed string, want ...Tuning) {
		t.Helper()
		got := SuggestTunings(ts, typed)
		if got == nil {
			t.Errorf("SuggestTunings(%q) = nil", typed)
		}
		if !slices.Equal(got, want) && !(len(got) == 0 && len(want) == 0) {
			t.Errorf("SuggestTunings(%q) = %v, want %v", typed, got, want)
		}
	}
	check("", ts...)
	check("drop", ts[0], ts[4])
	check("DROP C", ts[0], ts[4])
	check("c g c", ts[0], ts[4])       // by notes
	check("b e a", ts[1], ts[3])       // custom label is its notes
	check("  standard ", ts[2], ts[3]) // trimmed
	check("zzz")
	check("e a d g b e", ts[2], ts[3]) // also inside the 7-string notes
	if got := SuggestTunings(nil, "x"); got == nil || len(got) != 0 {
		t.Errorf("nil input = %#v", got)
	}
	t.Run("capped, grouping kept", func(t *testing.T) {
		var many []Tuning
		for i := range 100 {
			many = append(many, Tuning{Strings: 6 + i/50, Name: fmt.Sprintf("T%03d", i), Notes: "E"})
		}
		got := SuggestTunings(many, "")
		if len(got) != MaxSuggestions || got[0].Name != "T000" || got[59].Name != "T059" {
			t.Errorf("len %d, first %v", len(got), got[0])
		}
	})
}
