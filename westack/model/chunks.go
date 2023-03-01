package model

import (
	"fmt"
	"io"

	"github.com/mailru/easyjson"
)

type Chunk struct {
	raw    []byte
	first  bool
	last   bool
	error  error
	length int
}

type ChunkGenerator interface {
	ContentType() string

	GenerateNextChunk() bool
	NextChunk() (Chunk, error)
}

type InstanceAChunkGenerator struct {
	input InstanceA

	chunks       []Chunk
	totalChunks  int
	currentChunk int
	contentType  string
	debug        bool
}

func (chunkGenerator *InstanceAChunkGenerator) ContentType() string {
	return chunkGenerator.contentType
}

func (chunkGenerator *InstanceAChunkGenerator) NextChunk() (chunk Chunk, err error) {
	didGenerateChunk := chunkGenerator.GenerateNextChunk()
	if chunkGenerator.currentChunk == chunkGenerator.totalChunks {
		if !didGenerateChunk {
			return chunk, io.EOF
		}
	} else if chunkGenerator.currentChunk > chunkGenerator.totalChunks {
		return chunk, io.ErrUnexpectedEOF
	}
	chunk = chunkGenerator.chunks[chunkGenerator.currentChunk]
	if chunkGenerator.currentChunk == 0 {
		chunk.first = true
	} else if chunkGenerator.currentChunk == chunkGenerator.totalChunks-1 {
		chunk.last = true
	}
	chunkGenerator.currentChunk++
	return chunk, nil
}

func (chunkGenerator *InstanceAChunkGenerator) GenerateNextChunk() bool {
	if chunkGenerator.currentChunk == chunkGenerator.totalChunks {
		//fmt.Printf("ERROR: ChunkGenerator.GenerateNextChunk() called after EOF\n")
		return false
	}
	var nextChunk Chunk
	if chunkGenerator.currentChunk == 0 {
		nextChunk.raw = []byte{'['}
		nextChunk.length += 1
	} else {
		nextChunk.raw = []byte{','}
		nextChunk.length += 1
	}

	nextInstance := chunkGenerator.input[chunkGenerator.currentChunk]
	nextInstance.HideProperties()
	asM := nextInstance.ToJSON()
	asBytes, err := easyjson.Marshal(asM)
	if err != nil {
		nextChunk.error = err
		return false
	}
	nextChunk.raw = append(nextChunk.raw, asBytes...)
	nextChunk.length += len(asBytes)

	if chunkGenerator.currentChunk == chunkGenerator.totalChunks-1 {
		nextChunk.raw = append(nextChunk.raw, ']')
		nextChunk.length += 1
	}

	chunkGenerator.chunks = append(chunkGenerator.chunks, nextChunk)
	//fmt.Printf("Generated chunk %d/%d\n", chunkGenerator.currentChunk, chunkGenerator.totalChunks)
	return true
}

func (chunkGenerator *InstanceAChunkGenerator) Reader(eventContext *EventContext) io.Reader {
	return &ChunkGeneratorReader{
		chunkGenerator: chunkGenerator,
		eventContext:   eventContext,
		debug:          chunkGenerator.debug,
	}

}

type ChunkGeneratorReader struct {
	chunkGenerator        ChunkGenerator
	eventContext          *EventContext
	currentChunk          Chunk
	currentChunkReadIndex int
	finished              bool
	debug                 bool
}

func (reader *ChunkGeneratorReader) Read(p []byte) (n int, err error) {
	//if reader.currentChunk.length == 0 {
	//	reader.currentChunk, err = reader.chunkGenerator.NextChunk()
	//	if err != nil {
	//		return n, err
	//	}
	//	reader.currentChunkReadIndex = 0
	//}
	if reader.finished {
		return 0, io.EOF
	}
	if reader.debug {
		fmt.Printf("DEBUG: ChunkGeneratorReader.Read() called with len(p)=%d\n", len(p))
	}
	for i := 0; i < len(p); i++ {
		if reader.currentChunkReadIndex == reader.currentChunk.length {
			if reader.debug {
				fmt.Printf("DEBUG: ChunkGeneratorReader.Read() reached end of chunk (%d, %d, %d)\n", i, reader.currentChunkReadIndex, reader.currentChunk.length)
			}
			// stop reading from this chunk
			reader.currentChunk, err = reader.chunkGenerator.NextChunk()
			if err != nil {
				if err == io.EOF {
					if reader.debug {
						fmt.Printf("DEBUG: ChunkGeneratorReader.Read() reached EOF\n")
					}
					reader.finished = true
					return n, nil
				}
				fmt.Printf("ERROR: ChunkGeneratorReader.Read() failed to get next chunk: %v\n", err)
				return n, err
			}
			reader.currentChunkReadIndex = 0
		}
		p[i] = reader.currentChunk.raw[reader.currentChunkReadIndex]
		reader.currentChunkReadIndex++
		n++
	}
	if reader.debug {
		fmt.Printf("DEBUG: ChunkGeneratorReader.Read() returning %d bytes\n", n)
	}

	return n, nil
}

func NewInstanceAChunkGenerator(loadedModel *Model, input InstanceA, contentType string) *InstanceAChunkGenerator {
	result := InstanceAChunkGenerator{
		contentType:  contentType,
		chunks:       []Chunk{},
		currentChunk: 0,
		totalChunks:  len(input),
		input:        input,
		debug:        loadedModel.App.Debug,
	}
	return &result
}
