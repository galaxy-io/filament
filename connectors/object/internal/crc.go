// Package object contains provider-neutral support for object-store sinks.
package object

import "hash/crc32"

// CombineCRC32C returns the CRC32C of a || b from their individual checksums.
// This is the GF(2) matrix operation used by zlib's crc32_combine. It lets an
// object checksum reuse the checksums emitted by the streaming encoders.
func CombineCRC32C(a, b uint32, bLen int64) uint32 {
	if bLen <= 0 {
		return a
	}
	var even, odd [32]uint32
	odd[0] = crc32.Castagnoli
	row := uint32(1)
	for i := 1; i < len(odd); i++ {
		odd[i] = row
		row <<= 1
	}
	gf2MatrixSquare(&even, &odd)
	gf2MatrixSquare(&odd, &even)
	for {
		gf2MatrixSquare(&even, &odd)
		if bLen&1 != 0 {
			a = gf2MatrixTimes(&even, a)
		}
		bLen >>= 1
		if bLen == 0 {
			break
		}
		gf2MatrixSquare(&odd, &even)
		if bLen&1 != 0 {
			a = gf2MatrixTimes(&odd, a)
		}
		bLen >>= 1
		if bLen == 0 {
			break
		}
	}
	return a ^ b
}

func gf2MatrixTimes(matrix *[32]uint32, vector uint32) uint32 {
	var sum uint32
	index := 0
	for vector != 0 {
		if vector&1 != 0 {
			sum ^= matrix[index]
		}
		vector >>= 1
		index++
	}
	return sum
}

func gf2MatrixSquare(square, matrix *[32]uint32) {
	for i := range square {
		square[i] = gf2MatrixTimes(matrix, matrix[i])
	}
}
