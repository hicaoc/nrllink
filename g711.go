package main

const (
	SEG_MASK   = 0x70 // 0b01110000
	QUANT_MASK = 0x0F // 0b00001111
	SEG_SHIFT  = 4
	BIAS       = 0x84 //

)

var (
	alaw2linearTable [256]int16
	linear2alawTable [65536]byte
)

func init() {
	// Initialize Alaw to Linear table
	for i := range 256 {
		alaw2linearTable[i] = rawAlaw2linear(byte(i))
	}

	// Initialize Linear to Alaw table
	for i := range 65536 {
		linear2alawTable[i] = rawLinear2Alaw(int16(i))
	}
}

func alaw2linear(code byte) int16 {
	return alaw2linearTable[code]
}

func Linear2Alaw(sample int16) byte {
	return linear2alawTable[uint16(sample)]
}

func rawAlaw2linear(code byte) int16 {
	code ^= 0x55

	iexp := int16((code & 0x70) >> 4)
	mant := int16(code & 0x0F)

	if iexp > 0 {
		mant += 16
	}

	mant = (mant << 4) + 0x08

	if iexp > 1 {
		mant <<= (iexp - 1)
	}

	if (code & 0x80) != 0 {
		return mant
	}
	return -mant
}

func rawLinear2Alaw(sample int16) byte {
	var sign byte
	var ix int16

	if sample < 0 {
		sign = 0x80
		ix = (^sample) >> 4 // ✅ 按位取反
	} else {
		ix = sample >> 4
	}

	if ix > 15 {
		iexp := byte(1)
		for ix > 31 {
			ix >>= 1
			iexp++
		}
		ix -= 16
		ix += int16(iexp << 4)
	}

	if sign == 0 {
		ix |= 0x80
	}

	return byte(ix) ^ 0x55
}

// G711Encode converts a slice of 16-bit linear PCM samples to a slice of 8-bit A-law samples.
func G711Encode(pcmData []int) []byte {
	encoded := make([]byte, len(pcmData))
	for i := range pcmData {
		encoded[i] = Linear2Alaw(int16(pcmData[i]))
	}
	return encoded
}

func G711Decode(encodedData []byte) []int {
	decoded := make([]int, len(encodedData))
	for i := range encodedData {
		decoded[i] = int(alaw2linear(encodedData[i]))
	}
	return decoded
}
