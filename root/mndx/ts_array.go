package mndx

import (
	"encoding/binary"
	"io"

	"github.com/pkg/errors"
)

type triplet struct {
	BaseValue uint32
	Value2    uint32
	Value3    uint32
}

type tsArray struct {
	ItemBits       []uint32  // Bit array for each item (1 = item is present)
	TotalItemCount uint32    // Total number of items in the array
	ValidItemCount uint32    // Number of present items in the array
	BaseValues     []triplet // Array of base values for item indexes >= 0x200
	ArrayDwords38  []uint32
	ArrayDwords50  []uint32
}

func readUint32Array(r io.Reader, arr *[]uint32) error {
	var size uint64
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return errors.WithStack(err)
	}
	number := size / 4
	array := make([]uint32, number)
	if err := binary.Read(r, binary.LittleEndian, &arr); err != nil {
		return errors.WithStack(err)
	}
	arr = &array
	// reader.BaseStream.Position += (0 - numBytes) & 0x07;
	return nil
}

func readTripletArray(r io.Reader, arr *[]triplet) error {
	var size uint64
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return errors.WithStack(err)
	}
	number := size / (4 * 3)
	array := make([]triplet, number)
	if err := binary.Read(r, binary.LittleEndian, &arr); err != nil {
		return errors.WithStack(err)
	}
	arr = &array
	// reader.BaseStream.Position += (0 - numBytes) & 0x07;
	return nil
}

func parseTsArray(r io.Reader) (tsArray, error) {
	ts := tsArray{}
	if err := readUint32Array(r, &ts.ItemBits); err != nil {
		return tsArray{}, err
	}
	if err := binary.Read(r, binary.LittleEndian, &ts.TotalItemCount); err != nil {
		return tsArray{}, errors.WithStack(err)
	}
	if err := binary.Read(r, binary.LittleEndian, &ts.ValidItemCount); err != nil {
		return tsArray{}, errors.WithStack(err)
	}
	if err := readTripletArray(r, &ts.BaseValues); err != nil {
		return tsArray{}, err
	}
	if err := readUint32Array(r, &ts.ArrayDwords38); err != nil {
		return tsArray{}, err
	}
	if err := readUint32Array(r, &ts.ArrayDwords50); err != nil {
		return tsArray{}, err
	}
	return ts, nil
}

// public class TSparseArray
//     {
//         private int[] ItemBits;                    // Bit array for each item (1 = item is present)
//         private TRIPLET[] BaseValues;              // Array of base values for item indexes >= 0x200
//         private int[] ArrayDwords_38;
//         private int[] ArrayDwords_50;

//         public int TotalItemCount { get; private set; } // Total number of items in the array
//         public int ValidItemCount { get; private set; } // Number of present items in the array

//         public TSparseArray(BinaryReader reader)
//         {
//             ItemBits = reader.ReadArray<int>();
//             TotalItemCount = reader.ReadInt32();
//             ValidItemCount = reader.ReadInt32();
//             BaseValues = reader.ReadArray<TRIPLET>();
//             ArrayDwords_38 = reader.ReadArray<int>();
//             ArrayDwords_50 = reader.ReadArray<int>();
//         }

//         public bool Contains(int index)
//         {
//             return (ItemBits[index >> 0x05] & (1 << (index & 0x1F))) != 0;
//         }

//         public int GetItemBit(int index)
//         {
//             return ItemBits[index];
//         }

//         public TRIPLET GetBaseValue(int index)
//         {
//             return BaseValues[index];
//         }

//         public int GetArrayValue_38(int index)
//         {
//             return ArrayDwords_38[index];
//         }

//         public int GetArrayValue_50(int index)
//         {
//             return ArrayDwords_50[index];
//         }

//         public int GetItemValue(int index)
//         {
//             TRIPLET pTriplet;
//             int DwordIndex;
//             int BaseValue;
//             int BitMask;

//             //
//             // Divide the low-8-bits index to four parts:
//             //
//             // |-----------------------|---|------------|
//             // |       A (23 bits)     | B |      C     |
//             // |-----------------------|---|------------|
//             //
//             // A (23-bits): Index to the table (60 bits per entry)
//             //
//             //    Layout of the table entry:
//             //   |--------------------------------|-------|--------|--------|---------|---------|---------|---------|-----|
//             //   |  Base Value                    | val[0]| val[1] | val[2] | val[3]  | val[4]  | val[5]  | val[6]  |  -  |
//             //   |  32 bits                       | 7 bits| 8 bits | 8 bits | 9 bits  | 9 bits  | 9 bits  | 9 bits  |5bits|
//             //   |--------------------------------|-------|--------|--------|---------|---------|---------|---------|-----|
//             //
//             // B (3 bits) : Index of the variable-bit value in the array (val[#], see above)
//             //
//             // C (32 bits): Number of bits to be checked (up to 0x3F bits).
//             //              Number of set bits is then added to the values obtained from A and B

//             // Upper 23 bits contain index to the table
//             pTriplet = BaseValues[index >> 0x09];
//             BaseValue = pTriplet.BaseValue;

//             // Next 3 bits contain the index to the VBR
//             switch (((index >> 0x06) & 0x07) - 1)
//             {
//                 case 0:     // Add the 1st value (7 bits)
//                     BaseValue += (pTriplet.Value2 & 0x7F);
//                     break;

//                 case 1:     // Add the 2nd value (8 bits)
//                     BaseValue += (pTriplet.Value2 >> 0x07) & 0xFF;
//                     break;

//                 case 2:     // Add the 3rd value (8 bits)
//                     BaseValue += (pTriplet.Value2 >> 0x0F) & 0xFF;
//                     break;

//                 case 3:     // Add the 4th value (9 bits)
//                     BaseValue += (pTriplet.Value2 >> 0x17) & 0x1FF;
//                     break;

//                 case 4:     // Add the 5th value (9 bits)
//                     BaseValue += (pTriplet.Value3 & 0x1FF);
//                     break;

//                 case 5:     // Add the 6th value (9 bits)
//                     BaseValue += (pTriplet.Value3 >> 0x09) & 0x1FF;
//                     break;

//                 case 6:     // Add the 7th value (9 bits)
//                     BaseValue += (pTriplet.Value3 >> 0x12) & 0x1FF;
//                     break;
//             }

//             //
//             // Take the upper 27 bits as an index to DWORD array, take lower 5 bits
//             // as number of bits to mask. Then calculate number of set bits in the value
//             // masked value.
//             //

//             // Get the index into the array of DWORDs
//             DwordIndex = (index >> 0x05);

//             // Add number of set bits in the masked value up to 0x3F bits
//             if ((index & 0x20) != 0)
//                 BaseValue += GetNumbrOfSetBits32(ItemBits[DwordIndex - 1]);

//             BitMask = (1 << (index & 0x1F)) - 1;
//             return BaseValue + GetNumbrOfSetBits32(ItemBits[DwordIndex] & BitMask);
//         }

//         private int GetNumberOfSetBits(int Value32)
//         {
//             Value32 = ((Value32 >> 1) & 0x55555555) + (Value32 & 0x55555555);
//             Value32 = ((Value32 >> 2) & 0x33333333) + (Value32 & 0x33333333);
//             Value32 = ((Value32 >> 4) & 0x0F0F0F0F) + (Value32 & 0x0F0F0F0F);

//             return (Value32 * 0x01010101);
//         }

//         private int GetNumbrOfSetBits32(int x)
//         {
//             return (GetNumberOfSetBits(x) >> 0x18);
//         }
//     }
