package main

const (
	SEG_MASK   = 0x70 // 0b01110000
	QUANT_MASK = 0x0F // 0b00001111
	SEG_SHIFT  = 4
	BIAS       = 0x84 //

)

func alaw2linear(code byte) int16 {
	code ^= 0x55

	sign := int16(code & 0x80)
	seg := (code & 0x70) >> 4
	quant := code & 0x0F

	var sample int16
	if seg == 0 {
		sample = int16(quant<<1) | 0x01
	} else {
		sample = (int16(quant<<1) | 0x21) << (seg - 1)
	}

	if sign != 0 {
		return sample << 3
	}
	return -(sample << 3)
}

// Linear2Alaw converts a 16-bit linear PCM sample to an 8-bit A-law sample.
func Linear2Alaw(sample int16) byte {
	var sign byte
	if sample < 0 {
		if sample == -32768 {
			sample = -32767
		}
		sample = -sample
		sign = 0x00
	} else {
		sign = 0x80
	}

	// 13-bit absolute value for A-law
	pcm := sample >> 3

	seg := byte(0)
	if pcm >= 32 {
		seg = 1
		t := int16(64)
		for seg < 7 && pcm >= t {
			t <<= 1
			seg++
		}
	}

	var mant byte
	if seg == 0 {
		mant = byte(pcm>>1) & 0x0F
	} else {
		mant = byte(pcm>>seg) & 0x0F
	}

	return (sign | (seg << 4) | mant) ^ 0x55
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
