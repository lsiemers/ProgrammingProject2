Filesystem Project README

Lukas Siemers

Project Overview
This filesystem project implements a basic virtual filesystem in Go, allowing file operations like create, read, write, and delete within a virtual disk represented by an array.

Key Details
Directory Files Format
Directory entries are stored as Folder structures, containing the names and node IDs of child files/directories.
iNode Details
iNode Size: Each Node structure represents an iNode in the filesystem, with a fixed size determined by the Node structure's fields. The exact size in bytes can vary based on the underlying system architecture due to alignment and padding.
iNode Location: iNodes begin at block index specified by superBlock.NodeStart in the virtual disk/array.
Allocation Bitmap
Block Allocation Bitmap Location: The block allocation bitmap starts at block index superBlock.BitmapStart.
iNode Allocation Bitmap Location: An additional bitmap for iNode allocation tracking starts at superBlock.NodeMapStart.
Virtual Disk/Array Structure
The virtual disk is represented by a 2D byte array named Disk, with dimensions [6010][BlockSize]byte, where BlockSize is a constant defining the size of each block in bytes.
The first few blocks of the array are reserved for metadata, including the superblock and bitmaps for block and iNode allocation.
Data blocks and iNode blocks follow these metadata blocks, with their starting points configurable through the superblock.
Capacity
Maximum iNodes: The filesystem supports up to MaxInodes iNodes, as defined by a constant.
Maximum Data Blocks: The number of data blocks is implicitly defined by the size of the Disk array minus the blocks reserved for metadata and inodes.
