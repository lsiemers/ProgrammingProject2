package filesystem

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"time"
)

// Constants for filesystem parameters
const (
	BlockSize   = 1024
	MaxInodes   = 120
	MaxFileName = 12
)

// Structs for various filesystem components
type Node struct {
	Valid     bool
	Directory bool
	Blocks    []int
	Created   time.Time
	Modified  time.Time
	ID        int
}

type Folder struct {
	Name     string
	NodeID   int
	Children []int
	Names    []string
}

type File struct {
	Name    string
	NodeID  int
	Content string
}

type superblock struct {
	NodeStart    int
	BitmapStart  int
	NodeMapStart int
	BlockStart   int
}

// Globals
var Disk [6010][BlockSize]byte
var BlockMap [6000]bool
var NodeMap [MaxInodes]bool
var Nodes [MaxInodes]Node
var EndOfBlockMap int
var EndOfNodeMap int
var EndOfNodes int
var LastNodeBlock int
var NextNode int

// Initialization and utility functions
func Initialize() {
	// Initialize the root node and root folder directly
	rootNode := Node{
		Valid:     true,
		Directory: true,
		Blocks:    []int{9, 0, 0, 0}, // Assume the first data block for the root directory is 9
		Created:   time.Now(),
		Modified:  time.Now(),
		ID:        1,
	}

	// Directly assign the root node to the Nodes array
	Nodes[1] = rootNode
	var superBlock superblock
	// Simplify bitmap initialization
	NodeMap[1] = true  // Only root node is initially valid
	BlockMap[9] = true // Block 9 is reserved for the root directory

	// Create metadata with updated offsets if necessary
	superBlock = superblock{
		NodeStart:    4,
		BitmapStart:  3,
		NodeMapStart: 2,
		BlockStart:   10, // Assume blocks start from 10, leaving space for additional metadata if needed
	}

	// Directly encode and write metadata, root directory, and bitmaps to disk
	writeObjectToDisk(0, superBlock)
	writeObjectToDisk(9, Folder{Name: "root", NodeID: 1})

	// Directly convert and write bitmaps to disk
	writeBitmapToDisk(1, NodeMap[:])
	writeBitmapToDisk(2, BlockMap[:])

	// Encode and write the entire Nodes array to the disk, starting at block specified by meta.NodeStart
	writeNodesArrayToDisk(superBlock.NodeStart, Nodes)
}

var superBlock superblock

func writeNodesArrayToDisk(startBlock int, nodes [MaxInodes]Node) {
	// Create a buffer to hold the encoded node array
	var buf bytes.Buffer

	// Create a new encoder that will write to the buffer
	enc := gob.NewEncoder(&buf)

	// Encode the nodes array; gob can encode any Go data structure
	if err := enc.Encode(nodes); err != nil {
		log.Fatalf("Error encoding node array: %v", err)
	}

	// Retrieve the encoded byte slice
	data := buf.Bytes()

	// Iterate over the data, writing it block by block to the disk
	for i := 0; i*BlockSize < len(data); i++ {
		// Calculate the start and end indices for the data slice to write
		start := i * BlockSize
		end := start + BlockSize
		if end > len(data) {
			end = len(data)
		}

		// Determine the current block index on the disk
		currentBlockIndex := startBlock + i
		if currentBlockIndex >= len(Disk) {
			log.Fatalf("Block index out of bounds: trying to write beyond the disk capacity")
			return
		}

		// Clear the block before writing to prevent old data from remaining
		for j := range Disk[currentBlockIndex] {
			Disk[currentBlockIndex][j] = 0
		}

		// Copy the slice of encoded node array data to the disk block
		copy(Disk[currentBlockIndex][:], data[start:end])
	}
}

func writeBitmapToDisk(blockIndex int, bitmap []bool) {
	// Convert the boolean slice (bitmap) to a byte slice for efficient storage
	bitmapBytes := boolsToBytes(bitmap)

	// Ensure the bitmap does not exceed the available space in a block
	if len(bitmapBytes) > BlockSize {
		log.Fatalf("Error: bitmap size exceeds block size")
	}

	// Clear the block before writing to ensure no old data remains
	for i := range Disk[blockIndex] {
		Disk[blockIndex][i] = 0
	}

	// Write the byte slice to the specified block on the disk
	copy(Disk[blockIndex][:], bitmapBytes)
}

// Conversion functions between boolean arrays and byte slices for bitmap storage

func boolsToBytes(t []bool) []byte { //Thomas Provided this one
	b := make([]byte, (len(t)+7)/8)
	for i, x := range t {
		if x {
			b[i/8] |= 0x80 >> uint(i%8)
		}
	}
	return b
}

func bytesToBools(b []byte) []bool { //Thomas Provided this one
	t := make([]bool, 8*len(b))
	for i, x := range b {
		for j := 0; j < 8; j++ {
			if (x<<uint(j))&0x80 == 0x80 {
				t[8*i+j] = true
			}
		}
	}
	return t
}

// Additional filesystem operations, adapted and renamed where necessary
func ReadSuperblock() superblock {
	var sb superblock
	reader := bytes.NewReader(Disk[0][:]) // Ensure this points to the correct block
	decoder := gob.NewDecoder(reader)
	if err := decoder.Decode(&sb); err != nil {
		log.Fatalf("Failed to read superblock: %v", err)
	}
	return sb
}

func ReadDir(first, second, third, fourth int) Folder {
	var directory Folder
	var blockData []byte

	// Aggregate data from specified blocks into blockData
	blockIndices := []int{first, second, third, fourth}
	for _, index := range blockIndices {
		if index != 0 { // Assuming 0 is used to indicate an unused block index
			blockData = append(blockData, Disk[index][:]...)
		}
	}

	// Decode blockData into directory
	decoder := gob.NewDecoder(bytes.NewReader(blockData))
	if err := decoder.Decode(&directory); err != nil {
		log.Fatalf("Error decoding directory: %v", err)
	}

	return directory
}

func ReadNodeArray() [MaxInodes]Node {
	var nodes [MaxInodes]Node
	var blockData []byte

	meta := ReadSuperblock()
	for i := 0; i < LastNodeBlock; i++ {
		blockIndex := meta.NodeStart + i
		if len(Disk[blockIndex]) == 0 {
			log.Println("Encountered an empty data block before expected.")
			return nodes
		}
		blockData = append(blockData, Disk[blockIndex][:]...)
	}

	if len(blockData) == 0 {
		log.Println("No data found to decode.")
		return nodes
	}

	buf := bytes.NewBuffer(blockData)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(&nodes); err != nil {
		log.Fatalf("Failed to decode node array: %v", err)
	}

	return nodes
}

func writeObjectToDisk(blockIndex int, object interface{}) {
	// Create a buffer to hold the encoded data
	var buf bytes.Buffer

	// Create a new encoder that will write to the buffer
	enc := gob.NewEncoder(&buf)

	// Encode the object, gob can encode any type
	if err := enc.Encode(object); err != nil {
		log.Fatalf("Error encoding object to disk: %v", err)
	}

	// Retrieve the encoded byte slice
	data := buf.Bytes()

	// Ensure the data does not exceed the block size
	if len(data) > BlockSize {
		log.Fatalf("Error: encoded object size exceeds block size")
	}

	// Write the data to the specified block index in the Disk array
	// Make sure not to exceed the bounds of the Disk
	if blockIndex >= len(Disk) {
		log.Fatalf("Error: block index out of bounds")
	}

	// Clear the block before writing to prevent old data from remaining
	for i := range Disk[blockIndex] {
		Disk[blockIndex][i] = 0
	}

	// Copy the encoded data into the disk at the specified block index
	copy(Disk[blockIndex][:], data)
}
func WriteNodeArray(nodes [MaxInodes]Node) {
	var buf bytes.Buffer

	// Encode the nodes array into a byte buffer using gob
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(nodes); err != nil {
		log.Fatalf("Failed to encode node array: %v", err)
	}
	data := buf.Bytes()

	// Read metadata to find out where the node array should start on the disk
	meta := ReadSuperblock()

	// Calculate how many blocks are needed to store the entire node array
	totalNodeBlocks := (len(data) + BlockSize - 1) / BlockSize

	// Write the encoded node array to the disk, starting at the block index specified for nodes
	for i := 0; i < totalNodeBlocks; i++ {
		blockIndex := meta.NodeStart + i
		start := i * BlockSize
		end := start + BlockSize
		if end > len(data) {
			end = len(data)
		}

		// Ensure the blockIndex is within bounds
		if blockIndex >= len(Disk) {
			log.Fatalf("Block index out of bounds: %d", blockIndex)
			return
		}

		// Copy the slice of the encoded node array to the appropriate block on the disk
		copy(Disk[blockIndex][:], data[start:end])
	}

	// Optionally, update global variables related to the node storage, if any.
	// For instance, you might want to update EndOfNodes to reflect the new size of the node data on disk.
	EndOfNodes = len(data)
}

func UpdateBlockMap(blocks []bool) {
	// First, convert the slice of booleans to a slice of bytes
	bitmapBytes := boolsToBytes(blocks)

	// Read the metadata to find where the block bitmap is stored
	meta := ReadSuperblock()

	// Assuming the block bitmap is stored at a fixed location specified in the metadata
	blockBitmapStart := meta.BitmapStart

	// Ensure we're not trying to write beyond the disk size
	if blockBitmapStart+len(bitmapBytes) > len(Disk) {
		log.Fatalf("Error: Block bitmap exceeds disk size.")
	}

	// Write the byte representation of the block bitmap to the disk
	for i, byteVal := range bitmapBytes {
		Disk[blockBitmapStart+i][0] = byteVal
	}

	// Update global variables if necessary. For example, if you keep track of the end of the block bitmap
	EndOfBlockMap = blockBitmapStart + len(bitmapBytes) - 1
}

func UpdateNodeMap(nodes []bool) {
	// Convert the slice of booleans to a slice of bytes
	bitmapBytes := boolsToBytes(nodes)

	// Read the metadata to find out where the node bitmap is stored
	meta := ReadSuperblock()

	// Assuming the node bitmap is stored at a fixed location specified in the metadata
	nodeBitmapStart := meta.NodeMapStart

	// Ensure we're not trying to write beyond the disk size
	if nodeBitmapStart+len(bitmapBytes) > len(Disk) {
		log.Fatalf("Error: Node bitmap exceeds disk size.")
	}

	// Write the byte representation of the node bitmap to the disk
	for i, byteVal := range bitmapBytes {
		Disk[nodeBitmapStart+i][0] = byteVal
	}

	// Update global variables if necessary. For example, if you keep track of the end of the node bitmap
	EndOfNodeMap = nodeBitmapStart + len(bitmapBytes) - 1
}

func AddDir(d Folder, blocks [4]int) {
	// Encode the directory to a byte slice using gob
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(d); err != nil {
		log.Fatalf("Error encoding directory: %v", err)
	}
	data := buf.Bytes()

	// Write the encoded directory to the specified blocks on the disk
	dataIndex := 0 // Track our position in the encoded data

	for _, blockIndex := range blocks {
		// Check if we've written all data or if the block index is zero (indicating no more blocks)
		if dataIndex >= len(data) || blockIndex == 0 {
			break
		}

		// Calculate how much of the data can be written to the current block
		end := dataIndex + BlockSize
		if end > len(data) {
			end = len(data)
		}

		// Copy the data slice into the virtual disk block
		copy(Disk[blockIndex][:], data[dataIndex:end])

		// Update dataIndex to reflect the amount of data written

	}
}
func ReadFile(node Node) File {
	var file File
	var data []byte

	// Iterate through the node's data blocks to gather the file's data
	for _, blockIndex := range node.Blocks {
		// If the block index is zero, it indicates the end of the file data
		if blockIndex == 0 {
			break
		}
		// Append the data from the current block to the overall data slice
		data = append(data, Disk[blockIndex][:]...)
	}

	// Now decode the concatenated data into a File struct
	decoder := gob.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&file); err != nil {
		log.Fatalf("Error decoding file: %v", err)
	}

	return file
}

func CheckDirectoryForFile(inode Node, filename string) (bool, File) {
	if inode.Directory {
		dir := ReadDir(inode.Blocks[0], inode.Blocks[1], inode.Blocks[2], inode.Blocks[3])
		for _, name := range dir.Names {
			if name == filename {
				// Assume ReadFile takes a Node and returns a File
				return true, ReadFile(inode)
			}
		}
	}
	return false, File{}
}

func AllocateBlock() int {
	// Iterate over the BlockMap to find a free block (false indicates free)
	for i, used := range BlockMap {
		if !used {
			BlockMap[i] = true               // Mark as used
			UpdateBlockMap(BlockMap[:])      // Reflect changes on the disk
			return i + superBlock.BlockStart // Return the actual block index considering the offset
		}
	}
	return -1 // Indicate failure to allocate block
}

func WriteFile(f File, node Node) Node {
	// Encode the file to a byte slice using gob
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(f); err != nil {
		log.Fatalf("Error encoding file: %v", err)
	}
	data := buf.Bytes()

	// Calculate the number of blocks needed to store the file data
	numBlocksNeeded := (len(data) + BlockSize - 1) / BlockSize

	// Prepare to write the data to the disk
	dataIndex := 0  // Track our position in the encoded data
	blocksUsed := 0 // Track the number of blocks used

	// Ensure the node has enough blocks to store the file; allocate if necessary
	for blocksUsed < numBlocksNeeded {
		var blockIndex int
		if blocksUsed < len(node.Blocks) && node.Blocks[blocksUsed] != 0 {
			// Use an existing block
			blockIndex = node.Blocks[blocksUsed]
		} else {
			// Allocate a new block if necessary
			blockIndex = AllocateBlock() // Implement this function to find and mark a free block as used
			if blockIndex == -1 {
				log.Fatalf("Insufficient disk space to write file")
			}
			if blocksUsed < len(node.Blocks) {
				node.Blocks[blocksUsed] = blockIndex
			} else {
				log.Fatalf("File exceeds maximum block allocation")
			}
		}

		// Calculate how much data to write to the current block
		end := dataIndex + BlockSize
		if end > len(data) {
			end = len(data)
		}

		// Write the slice of the encoded file to the disk block
		copy(Disk[blockIndex][:], data[dataIndex:end])

		// Update our tracking variables
		dataIndex = end
		blocksUsed++
	}

	// Return the updated node with potentially new blocks allocated
	return node
}
func CreateNewFile(filename string, parentDirectoryNodeID int) {
	// Check if the filename exceeds the maximum length allowed
	if len(filename) > MaxFileName {
		log.Printf("Filename '%s' exceeds the maximum allowed length of %d characters.\n", filename, MaxFileName)
		return
	}

	// Read the parent directory to see if the file already exists
	parentDir := ReadDirFromNodeID(parentDirectoryNodeID)
	for _, name := range parentDir.Names {
		if name == filename {
			log.Printf("A file with name '%s' already exists in the directory with node ID %d.\n", filename, parentDirectoryNodeID)
			return
		}
	}

	// Allocate a new node for the file
	newNodeID := AllocateNode()
	if newNodeID == -1 {
		log.Println("Failed to allocate a new node for the file.")
		return
	}

	// Initialize the file node
	newFileNode := Node{
		Valid:     true,
		Directory: false,
		Blocks:    []int{0, 0, 0, 0}, // Initially, no data blocks are allocated
		Created:   time.Now(),
		Modified:  time.Now(),
		ID:        newNodeID,
	}

	// Update the parent directory node to include this new file
	parentDir.Children = append(parentDir.Children, newNodeID)
	parentDir.Names = append(parentDir.Names, filename)

	// Write the new file node and updated directory back to the disk
	WriteNodeToDisk(newFileNode)
	WriteDirToNodeID(parentDirectoryNodeID, parentDir)

	log.Printf("File '%s' created successfully with node ID %d in directory node ID %d.\n", filename, newNodeID, parentDirectoryNodeID)
}
func ReadDirFromNodeID(nodeID int) Folder {
	var folder Folder

	// Assuming each Node's Blocks[0] (if it's a directory) points to the block where the directory data starts.
	dirBlock := Nodes[nodeID].Blocks[0]
	if dirBlock == 0 {
		log.Printf("Directory node %d does not have any associated data blocks.\n", nodeID)
		return folder
	}

	// Read the directory information from the disk
	data := Disk[dirBlock][:]
	reader := bytes.NewReader(data)
	decoder := gob.NewDecoder(reader)
	if err := decoder.Decode(&folder); err != nil {
		log.Fatalf("Failed to read directory from node ID %d: %v\n", nodeID, err)
	}

	return folder
}
func WriteNodeToDisk(node Node) {
	// Nodes are written to a pre-defined area on the disk, based on their ID
	startBlock := superBlock.NodeStart + node.ID // Example calculation, adjust based on your layout

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(node); err != nil {
		log.Fatalf("Failed to encode node %d: %v", node.ID, err)
	}

	// Ensure you don't exceed your disk or nodes area capacity
	if startBlock >= len(Disk) {
		log.Fatalf("Node start block %d is out of disk bounds.", startBlock)
	}

	copy(Disk[startBlock][:], buf.Bytes())
}
func WriteDirToNodeID(nodeID int, folder Folder) {
	dirBlock := Nodes[nodeID].Blocks[0] // Assuming the first block is where the directory data starts
	if dirBlock == 0 {
		log.Printf("Directory node %d does not have an allocated block.\n", nodeID)
		return
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(folder); err != nil {
		log.Fatalf("Failed to encode directory for node ID %d: %v", nodeID, err)
	}

	// Write back to the same block
	copy(Disk[dirBlock][:], buf.Bytes())
}
func AllocateNode() int {
	for i, used := range NodeMap {
		if !used {
			NodeMap[i] = true         // Mark the node as used
			UpdateNodeMap(NodeMap[:]) // Reflect this change back on the disk

			// Initialize the Node to avoid stale data
			Nodes[i] = Node{ID: i, Valid: true}

			return i // Return the allocated node ID
		}
	}
	log.Println("Failed to allocate a new node: no space left.")
	return -1 // Indicates failure to allocate a new node
}
func FindFileNode(filename string, directoryNodeID int) (Node, bool) {
	// First, read the directory's content based on the provided directoryNodeID
	directory := ReadDirFromNodeID(directoryNodeID)

	// Iterate through the directory's file names to find a match for the filename
	for index, name := range directory.Names {
		if name == filename {
			// If a match is found, get the corresponding node ID from the directory's slice
			fileNodeID := directory.Children[index]

			// fetch the Node struct
			if fileNodeID < len(Nodes) && Nodes[fileNodeID].Valid {
				// Return the Node struct and true indicating the file was found
				return Nodes[fileNodeID], true
			}
		}
	}

	// If no matching filename is found in the directory, return an empty Node struct and false
	return Node{}, false
}
func WriteToFile(fileNode Node, data string) {
	// Convert the string data to bytes
	dataBytes := []byte(data)

	// Check if existing blocks are enough to store the data
	totalBlocksNeeded := (len(dataBytes) + BlockSize - 1) / BlockSize

	// Allocate new blocks if needed
	for len(fileNode.Blocks) < totalBlocksNeeded {
		newBlock := AllocateBlock()
		if newBlock == -1 {
			log.Println("No space left on device to write data.")
			return
		}
		fileNode.Blocks = append(fileNode.Blocks, newBlock+superBlock.BlockStart) // Adjust based on your block allocation strategy
	}

	// Write data to blocks

	currentByteIndex := 0
	for i := 0; i < totalBlocksNeeded; i++ {
		blockIndex := fileNode.Blocks[i]
		endByteIndex := currentByteIndex + BlockSize
		if endByteIndex > len(dataBytes) {
			endByteIndex = len(dataBytes)
		}
		blockData := dataBytes[currentByteIndex:endByteIndex]
		copy(Disk[blockIndex][:], blockData)
		currentByteIndex += BlockSize
	}

	// Update the file node's modification time
	fileNode.Modified = time.Now()

	// Write the updated file node back to the disk
	WriteNodeToDisk(fileNode)

	log.Printf("Data written to file node ID %d\n", fileNode.ID)
}
func ReadFromFile(fileNode Node) string {
	var fileData []byte

	// Iterate over each block allocated to the file
	for _, blockIndex := range fileNode.Blocks {
		// Check if the block index is valid. Assuming 0 is an invalid block index.
		if blockIndex == 0 {
			break // Reached the end of the allocated blocks for this file
		}
		// Append the data from the block to the fileData slice
		// Assuming Disk is a global array representing the disk blocks
		fileData = append(fileData, Disk[blockIndex][:]...)
	}

	// If the file's actual data is smaller than the allocated blocks
	nulIndex := bytes.IndexByte(fileData, 0)
	if nulIndex != -1 {
		fileData = fileData[:nulIndex]
	}

	// Convert the file data from bytes to a string and return
	return string(fileData)
}
func AppendToFile(fileNode Node, data string) {
	// Convert the append data to bytes
	appendDataBytes := []byte(data)

	// Calculate the total size of the new data including existing content
	totalDataSize := len(appendDataBytes)
	for _, blockIndex := range fileNode.Blocks {
		if blockIndex != 0 {
			// Assuming each block is fully used by the file before appending new data
			totalDataSize += BlockSize
		}
	}

	// Calculate total blocks needed for the new size
	totalBlocksNeeded := (totalDataSize + BlockSize - 1) / BlockSize

	// Check if new blocks need to be allocated
	if totalBlocksNeeded > len(fileNode.Blocks) {
		for i := len(fileNode.Blocks); i < totalBlocksNeeded; i++ {
			newBlock := AllocateBlock()
			if newBlock == -1 {
				log.Println("No space left on device to append data.")
				return
			}
			fileNode.Blocks = append(fileNode.Blocks, newBlock+superBlock.BlockStart) // Adjust based on your block allocation strategy
		}
	}

	// Append data to the last block of the file, or new blocks if allocated
	lastBlockIndex := fileNode.Blocks[len(fileNode.Blocks)-1]
	// Assuming Disk is a global array representing the disk blocks
	lastBlockData := Disk[lastBlockIndex][:]
	nulIndex := bytes.IndexByte(lastBlockData, 0)
	if nulIndex == -1 {
		// Last block is fully used, append data starting from a new block
		nulIndex = BlockSize
	}
	remainingSpace := BlockSize - nulIndex
	if remainingSpace > len(appendDataBytes) {
		// All append data fits into the remaining space of the last block
		copy(Disk[lastBlockIndex][nulIndex:], appendDataBytes)
	} else {
		// Fill up the remaining space of the last block
		copy(Disk[lastBlockIndex][nulIndex:], appendDataBytes[:remainingSpace])
		appendDataBytes = appendDataBytes[remainingSpace:]

		// Write the rest of the data to new blocks
		for _, blockIndex := range fileNode.Blocks[len(fileNode.Blocks)-totalBlocksNeeded+len(fileNode.Blocks):] {
			if len(appendDataBytes) <= BlockSize {
				copy(Disk[blockIndex][:], appendDataBytes)
				break
			} else {
				copy(Disk[blockIndex][:], appendDataBytes[:BlockSize])
				appendDataBytes = appendDataBytes[BlockSize:]
			}
		}
	}

	// Update the file node's modification time
	fileNode.Modified = time.Now()

	// Write the updated file node back to the disk
	WriteNodeToDisk(fileNode)

	log.Printf("Data appended to file node ID %d\n", fileNode.ID)
}

func Open(mode string, filename string, searchnode int) {

	if mode == "open" {
		inodes := ReadNodeArray()
		var found bool
		for _, inode := range inodes {
			if inode.ID == searchnode && inode.Valid {
				// Assuming a function that checks if the inode corresponds to the directory and contains the file
				found, _ = CheckDirectoryForFile(inode, filename)
				break
			}
		}
		if !found {
			fmt.Println("File not found, creating new file:", filename)
			CreateNewFile(filename, searchnode)
		} else {
			fmt.Println("File opened:", filename)
		}
	}

	if mode == "write" {
		// Assuming a function that finds the file's node based on filename and directory node ID
		// Find or create the file node
		fileNode, found := FindFileNode(filename, searchnode)
		if !found {
			fmt.Println("File not found, creating new file:", filename)
			CreateNewFile(filename, searchnode)
			fileNode, found = FindFileNode(filename, searchnode)
			if !found {
				log.Println("Failed to create or find the file after creation.")
				return
			}
		}

		// Write data to the file
		fmt.Println("Writing to file:", filename)
		WriteToFile(fileNode) // Assumes WriteToFile handles block allocation
	}
	if mode == "read" {
		inodes := ReadNodeArray() // Assume this returns all inodes in the filesystem
		var fileFound bool
		var fileContent string
		var fileNode Node // Assuming a struct that represents a file node

		// Search for the file in the specified directory inode
		for _, inode := range inodes {
			if inode.ID == searchNode && inode.Valid {
				found, tmpNode := CheckDirectoryForFile(inode, filename) // Assuming this returns bool and Node
				if found {
					fileNode = tmpNode
					fileFound = true
					break
				}
			}
		}

		if fileFound {
			// Read the file content
			fileContent = ReadFromFile(fileNode) // Assumes ReadFromFile returns the content of the file as a string
			fmt.Println("File content:", fileContent)
		} else {
			fmt.Println("File not found:", filename)
		}
	}
	if mode == "append" {
		fileNode, found := FindFileNode(filename, searchnode)
		if found {
			fmt.Println("Appending to file:", filename)
			// Assuming a function that appends data to the file; data input is omitted for simplicity
			AppendToFile(fileNode, "Data to append")
		} else {
			fmt.Println("File not found:", filename)
		}
	}
}
