// Package shortcode 负责生成六位短码。
//
// 短码的字符集是数字 0 到 9 加上小写字母 a 到 z 加上大写字母 A 到 Z，一共 62 个字符，
// 因此长度为 6 的短码一共有 62 的 6 次方种取值，约等于 5.7 乘以 10 的 10 次方。
package shortcode

import (
	"crypto/rand"
	"math/big"
)

// Alphabet 是短码使用的字符集。
// 使用这个取值，是为了让短码可以直接出现在网址路径中而不需要做百分号编码。
const Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Length 是短码的固定长度。
// 接口契约与 nginx 中的正则规则都依赖这个取值，因此修改它之前必须同时修改这两处。
const Length = 6

// Generate 生成一个长度为 Length 的短码。
//
// 实现要求：
//
//  1. 返回值的长度必须等于 Length，且每一个字符都必须出现在 Alphabet 中。
//
//  2. 随机源必须使用 crypto/rand 而不是 math/rand。
//     原因有两层：第一层是 math/rand 在未设置种子的情况下每次启动产生相同的序列，
//     因此重建 Pod 之后生成的短码会重复；第二层是短码会出现在公开网址里，
//     使用可预测的随机源会让任何人都能猜到尚未被创建的短码，进而批量占位。
//
//  3. 不要把 Rand.IntN 这类函数与取模运算组合使用，因为取模会引入取模偏差：
//     当随机数的取值范围不能被 62 整除时，位置靠前的字符被选中的概率会偏高。
//     使用 crypto/rand.Int 并且把上界设置为字符集的长度，可以直接避免这个问题。
//
// 本函数使用 crypto/rand 与 math/big 两个包，这两个包的 API 与用法见 GO-CHEATSHEET.md 第 14 节。
// 验收方式：执行 go test ./internal/shortcode/... 通过，测试文件已经写好。
func Generate() (string, error) {
	ret := make([]byte, 0, Length)
	upper := big.NewInt(int64(len(Alphabet)))

	for range Length {
		index, err := rand.Int(rand.Reader, upper)
		if err != nil {
			return "", err
		}
		ret = append(ret, Alphabet[index.Int64()])
	}

	return string(ret), nil
}
