package mndx

import "io"

type marFile struct {
}

func parseMarFile(r io.Reader) (marFile, error) {
	marFile := marFile{}
	marFile.struct68_00 = tsParseArray(r)
	// Struct68_00 = new TSparseArray(reader);
	// FileNameIndexes = new TSparseArray(reader);
	// Struct68_D0 = new TSparseArray(reader);
	// FrgmDist_LoBits = reader.ReadArray<byte>();
	// FrgmDist_HiBits = new TBitEntryArray(reader);
	// IndexStruct_174 = new TNameIndexStruct(reader);

	// if (Struct68_D0.ValidItemCount != 0 && IndexStruct_174.Count == 0)
	// {
	// 	NextDB = new MARFileNameDB(reader, true);
	// }

	// NameFragTable = reader.ReadArray<NAME_FRAG>();

	// NameFragIndexMask = NameFragTable.Length - 1;

	// field_214 = reader.ReadInt32();

	// int dwBitMask = reader.ReadInt32();
}

// class MARFileNameDB
//     {
//         private const int CASC_MAR_SIGNATURE = 0x0052414d;           // 'MAR\0'

//         private TSparseArray Struct68_00;
//         private TSparseArray FileNameIndexes;
//         private TSparseArray Struct68_D0;
//         private byte[] FrgmDist_LoBits;
//         private TBitEntryArray FrgmDist_HiBits;
//         private TNameIndexStruct IndexStruct_174;
//         private MARFileNameDB NextDB;
//         private NAME_FRAG[] NameFragTable;
//         private int NameFragIndexMask;
//         private int field_214;

//         public int NumFiles { get { return FileNameIndexes.ValidItemCount; } }

//         private byte[] table_1BA1818 =
//         {
//              0x07, 0x07, 0x07, 0x07, 0x07, 0x07, 0x07,...
//         };

//         public MARFileNameDB(BinaryReader reader, bool next = false)
//         {
//             if (!next && reader.ReadInt32() != CASC_MAR_SIGNATURE)
//                 throw new Exception("invalid MAR file");

//             Struct68_00 = new TSparseArray(reader);
//             FileNameIndexes = new TSparseArray(reader);
//             Struct68_D0 = new TSparseArray(reader);
//             FrgmDist_LoBits = reader.ReadArray<byte>();
//             FrgmDist_HiBits = new TBitEntryArray(reader);
//             IndexStruct_174 = new TNameIndexStruct(reader);

//             if (Struct68_D0.ValidItemCount != 0 && IndexStruct_174.Count == 0)
//             {
//                 NextDB = new MARFileNameDB(reader, true);
//             }

//             NameFragTable = reader.ReadArray<NAME_FRAG>();

//             NameFragIndexMask = NameFragTable.Length - 1;

//             field_214 = reader.ReadInt32();

//             int dwBitMask = reader.ReadInt32();
//         }
