package shortcode

import (
	"strings"
	"testing"
)

// TestGenerateLengthAndAlphabet 检查生成的短码在长度与字符集这两个方面是否满足要求。
func TestGenerateLengthAndAlphabet(t *testing.T) {
	// 生成 1000 个短码，逐一检查。数量取 1000 是因为单次生成的结果无法覆盖字符集，
	// 而 1000 乘以 6 等于 6000 个字符，足以让字符集的每一个位置都被检验到。
	for i := 0; i < 1000; i++ {
		code, err := Generate()
		if err != nil {
			t.Fatalf("第 %d 次调用 Generate 返回错误：%v", i, err)
		}
		if len(code) != Length {
			t.Fatalf("第 %d 次调用 Generate 得到的短码 %q 长度是 %d，期望长度是 %d", i, code, len(code), Length)
		}
		for _, ch := range code {
			if !strings.ContainsRune(Alphabet, ch) {
				t.Fatalf("第 %d 次调用 Generate 得到的短码 %q 包含字符集之外的字符 %q", i, code, ch)
			}
		}
	}
}

// TestGenerateIsNotConstant 检查连续两次调用是否可能返回相同的结果。
// 本测试的目的不是严格证明随机性，而是排除「每次返回同一个固定值」这种最明显的实现错误。
func TestGenerateIsNotConstant(t *testing.T) {
	seen := make(map[string]struct{}, 200)
	for i := 0; i < 200; i++ {
		code, err := Generate()
		if err != nil {
			t.Fatalf("第 %d 次调用 Generate 返回错误：%v", i, err)
		}
		seen[code] = struct{}{}
	}
	// 62 的 6 次方约等于 5.7 乘以 10 的 10 次方，200 次生成中出现重复的概率可以忽略，
	// 因此只要出现重复就可以判定实现有问题。
	if len(seen) != 200 {
		t.Fatalf("生成 200 个短码之后只得到 %d 个互不相同的取值，说明随机源有问题", len(seen))
	}
}

// TestGenerateCoversAlphabet 检查大量生成之后字符集是否被充分覆盖。
// 本测试用来发现「只使用字符集中的一部分字符」这类实现错误，例如误用十六进制字符集。
func TestGenerateCoversAlphabet(t *testing.T) {
	counts := make(map[rune]int, len(Alphabet))
	const rounds = 500
	for i := 0; i < rounds; i++ {
		code, err := Generate()
		if err != nil {
			t.Fatalf("第 %d 次调用 Generate 返回错误：%v", i, err)
		}
		for _, ch := range code {
			counts[ch]++
		}
	}

	total := rounds * Length
	for _, ch := range Alphabet {
		if counts[ch] == 0 {
			t.Fatalf("生成 %d 个字符之后字符 %q 一次都没有出现，说明字符集没有被完整使用", total, ch)
		}
	}
}
