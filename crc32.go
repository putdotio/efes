// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package crc32 implements the 32-bit cyclic redundancy check, or CRC-32,
// checksum. See http://en.wikipedia.org/wiki/Cyclic_redundancy_check for
// information.
//
// Polynomials are represented in LSB-first form also known as reversed representation.
//
// See http://en.wikipedia.org/wiki/Mathematics_of_cyclic_redundancy_checks#Reversed_representations_and_reciprocal_polynomials
// for information.
package main

import "sync"

// nolint
// The size of a CRC-32 checksum in bytes.
const CRC32Size = 4

// Predefined polynomials.
const (
	// IEEE is by far and away the most common CRC-32 polynomial.
	// Used by ethernet (IEEE 802.3), v.42, fddi, gzip, zip, png, ...
	IEEE = 0xedb88320
)

// Table is a 256-word table representing the polynomial for efficient processing.
type Table [256]uint32

// IEEETable is the table for the IEEE polynomial.
var IEEETable = simpleMakeTable(IEEE)

// ieeeTable8 is the slicing8Table for IEEE
var ieeeTable8 *slicing8Table
var updateIEEE func(crc uint32, p []byte) uint32
var ieeeOnce sync.Once

func ieeeInit() {
	// Initialize the slicing-by-8 table.
	ieeeTable8 = slicingMakeTable(IEEE)
	updateIEEE = func(crc uint32, p []byte) uint32 {
		return slicingUpdate(crc, ieeeTable8, p)
	}
}

// crc32digest represents the partial evaluation of a checksum.
type crc32digest struct {
	crc uint32
	tab *Table
}

// nolint
// NewCRC32 creates a new hash.Hash32 computing the CRC-32 checksum
// using the polynomial represented by the Table.
// Its Sum method will lay the value out in big-endian byte order.
func NewCRC32(tab *Table) *crc32digest {
	if tab == IEEETable {
		ieeeOnce.Do(ieeeInit)
	}
	return &crc32digest{0, tab}
}

// nolint
// NewCRC32IEEE creates a new hash.Hash32 computing the CRC-32 checksum
// using the IEEE polynomial.
// Its Sum method will lay the value out in big-endian byte order.
func NewCRC32IEEE() *crc32digest { return NewCRC32(IEEETable) }

func (d *crc32digest) Size() int { return CRC32Size }

func (d *crc32digest) BlockSize() int { return 1 }

func (d *crc32digest) Reset() { d.crc = 0 }

func (d *crc32digest) Write(p []byte) (n int, err error) {
	switch d.tab {
	case IEEETable:
		// We only create digest objects through NewCRC32() which takes care of
		// initialization in this case.
		d.crc = updateIEEE(d.crc, p)
	default:
		d.crc = simpleUpdate(d.crc, d.tab, p)
	}
	return len(p), nil
}

func (d *crc32digest) Sum32() uint32 { return d.crc }

func (d *crc32digest) Sum(in []byte) []byte {
	s := d.Sum32()
	return append(in, byte(s>>24), byte(s>>16), byte(s>>8), byte(s))
}

// simpleMakeTable allocates and constructs a Table for the specified
// polynomial. The table is suitable for use with the simple algorithm
// (simpleUpdate).
func simpleMakeTable(poly uint32) *Table {
	t := new(Table)
	simplePopulateTable(poly, t)
	return t
}

// simplePopulateTable constructs a Table for the specified polynomial, suitable
// for use with simpleUpdate.
func simplePopulateTable(poly uint32, t *Table) {
	for i := 0; i < 256; i++ {
		crc := uint32(i)
		for j := 0; j < 8; j++ {
			if crc&1 == 1 {
				crc = (crc >> 1) ^ poly
			} else {
				crc >>= 1
			}
		}
		t[i] = crc
	}
}

// simpleUpdate uses the simple algorithm to update the CRC, given a table that
// was previously computed using simpleMakeTable.
func simpleUpdate(crc uint32, tab *Table, p []byte) uint32 {
	crc = ^crc
	for _, v := range p {
		crc = tab[byte(crc)^v] ^ (crc >> 8)
	}
	return ^crc
}

// Use slicing-by-8 when payload >= this value.
const slicing8Cutoff = 16

// slicing8Table is array of 8 Tables, used by the slicing-by-8 algorithm.
type slicing8Table [8]Table

// slicingMakeTable constructs a slicing8Table for the specified polynomial. The
// table is suitable for use with the slicing-by-8 algorithm (slicingUpdate).
func slicingMakeTable(poly uint32) *slicing8Table {
	t := new(slicing8Table)
	simplePopulateTable(poly, &t[0])
	for i := 0; i < 256; i++ {
		crc := t[0][i]
		for j := 1; j < 8; j++ {
			crc = t[0][crc&0xFF] ^ (crc >> 8)
			t[j][i] = crc
		}
	}
	return t
}

// slicingUpdate uses the slicing-by-8 algorithm to update the CRC, given a
// table that was previously computed using slicingMakeTable.
func slicingUpdate(crc uint32, tab *slicing8Table, p []byte) uint32 {
	if len(p) >= slicing8Cutoff {
		crc = ^crc
		for len(p) > 8 {
			crc ^= uint32(p[0]) | uint32(p[1])<<8 | uint32(p[2])<<16 | uint32(p[3])<<24
			crc = tab[0][p[7]] ^ tab[1][p[6]] ^ tab[2][p[5]] ^ tab[3][p[4]] ^
				tab[4][crc>>24] ^ tab[5][(crc>>16)&0xFF] ^
				tab[6][(crc>>8)&0xFF] ^ tab[7][crc&0xFF]
			p = p[8:]
		}
		crc = ^crc
	}
	if len(p) == 0 {
		return crc
	}
	return simpleUpdate(crc, &tab[0], p)
}
