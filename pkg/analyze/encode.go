package analyze

import (
	"encoding/json"
	"io"
	"strconv"
)

// EncodeJSON writes JSON representation of dir
func (f *Dir) EncodeJSON(writer io.Writer, topLevel bool) error {
	buff := make([]byte, 0, 20)

	buff = append(buff, []byte(`[{"name":`)...)

	if topLevel {
		if err := addString(&buff, f.GetPath()); err != nil {
			return err
		}
	} else {
		if err := addString(&buff, f.GetName()); err != nil {
			return err
		}
	}

	if !f.GetMtime().IsZero() {
		buff = append(buff, []byte(`,"mtime":`)...)
		buff = append(buff, []byte(strconv.FormatInt(f.GetMtime().Unix(), 10))...)
	}

	buff = append(buff, '}')
	if f.Files.Len() > 0 {
		buff = append(buff, ',')
	}
	buff = append(buff, '\n')

	if _, err := writer.Write(buff); err != nil {
		return err
	}

	for i, item := range f.Files {
		if i > 0 {
			if _, err := writer.Write([]byte(",\n")); err != nil {
				return err
			}
		}
		err := item.EncodeJSON(writer, false)
		if err != nil {
			return err
		}
	}

	if _, err := writer.Write([]byte("]")); err != nil {
		return err
	}
	return nil
}

// EncodeJSON writes JSON representation of file
func (f *File) EncodeJSON(writer io.Writer, topLevel bool) error {
	buff := make([]byte, 0, 20)

	buff = append(buff, []byte(`{"name":`)...)
	if err := addString(&buff, f.GetName()); err != nil {
		return err
	}
	if f.GetSize() > 0 {
		buff = append(buff, []byte(`,"asize":`)...)
		buff = append(buff, []byte(strconv.FormatInt(f.GetSize(), 10))...)
	}
	if f.GetUsage() > 0 {
		buff = append(buff, []byte(`,"dsize":`)...)
		buff = append(buff, []byte(strconv.FormatInt(f.GetUsage(), 10))...)
	}
	if !f.GetMtime().IsZero() {
		buff = append(buff, []byte(`,"mtime":`)...)
		buff = append(buff, []byte(strconv.FormatInt(f.GetMtime().Unix(), 10))...)
	}

	if f.Flag == '@' {
		buff = append(buff, []byte(`,"notreg":true`)...)
	}
	if f.Flag == 'H' {
		buff = append(buff, []byte(`,"ino":`+strconv.FormatUint(f.Mli, 10)+`,"hlnkc":true`)...)
	}

	buff = append(buff, '}')

	if _, err := writer.Write(buff); err != nil {
		return err
	}
	return nil
}

func addString(buff *[]byte, val string) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	*buff = append(*buff, b...)
	return err
}
