package main

// ratio mirrors Python's difflib.SequenceMatcher(None, a, b).ratio():
// 2*M/T with M the characters in Ratcliff/Obershelp matching blocks.
// Python's autojunk heuristic only applies to sequences of 200+ items and
// is not needed for album keys.
func ratio(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	if len(ra)+len(rb) == 0 {
		return 1
	}
	b2j := map[rune][]int{}
	for j, r := range rb {
		b2j[r] = append(b2j[r], j)
	}
	matches := 0
	var match func(alo, ahi, blo, bhi int)
	match = func(alo, ahi, blo, bhi int) {
		i, j, k := longestMatch(ra, b2j, alo, ahi, blo, bhi)
		if k == 0 {
			return
		}
		matches += k
		if alo < i && blo < j {
			match(alo, i, blo, j)
		}
		if i+k < ahi && j+k < bhi {
			match(i+k, ahi, j+k, bhi)
		}
	}
	match(0, len(ra), 0, len(rb))
	return 2 * float64(matches) / float64(len(ra)+len(rb))
}

// longestMatch finds the longest common block of a[alo:ahi] and b[blo:bhi],
// preferring the earliest start in a, then in b (as difflib does).
func longestMatch(a []rune, b2j map[rune][]int, alo, ahi, blo, bhi int) (besti, bestj, bestsize int) {
	besti, bestj = alo, blo
	j2len := map[int]int{}
	for i := alo; i < ahi; i++ {
		next := map[int]int{}
		for _, j := range b2j[a[i]] {
			if j < blo {
				continue
			}
			if j >= bhi {
				break
			}
			k := j2len[j-1] + 1
			next[j] = k
			if k > bestsize {
				besti, bestj, bestsize = i-k+1, j-k+1, k
			}
		}
		j2len = next
	}
	return besti, bestj, bestsize
}
