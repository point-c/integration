package archive

import "time"

type (
	FileHeader interface {
		entry()
		EntryName() string
		EntryTime() time.Time
	}
	entry[C any] interface {
		FileHeader
		EntryContent() C
	}
	Entry[C any] struct {
		Name    string
		Time    time.Time
		Content C
	}
)

func (Entry[C]) entry()                   {}
func (bce Entry[C]) EntryName() string    { return bce.Name }
func (bce Entry[C]) EntryTime() time.Time { return bce.Time }
func (bce Entry[C]) EntryContent() C      { return bce.Content }
