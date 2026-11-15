package analyze

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/dundee/gdu/v6/pkg/fs"
)

// EncodeJSON writes JSON representation of dir
func (f *Dir) EncodeJSON(writer io.Writer, topLevel bool, attributes fs.JSONAttributes) error {
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

	buff = appendDirSizes(buff, attributes, f.OwnUsage, f.GetSize(), f.GetUsage(), f.GetItemCount())
	if attributes.Includes("mtime") && !f.GetMtime().IsZero() {
		buff = append(buff, []byte(`,"mtime":`)...)
		buff = append(buff, []byte(strconv.FormatInt(f.GetMtime().Unix(), 10))...)
	}
	// The device is written where it changes, and inherited from the parent
	// elsewhere, as ncdu reads it. The top directory is left out, so an
	// export of one filesystem is the same whether it was scanned to memory
	// or to a database, which does not know devices; it is imported as 0.
	if dev := f.device(); attributes.Includes("dev") && !topLevel && f.Parent != nil && dev != Device(f.Parent) {
		buff = append(buff, []byte(`,"dev":`)...)
		buff = append(buff, []byte(strconv.FormatUint(dev, 10))...)
	}
	// ncdu's name for a directory that could not be read in full; without
	// it an export cannot be told apart from a complete scan.
	if f.Flag == '!' {
		buff = append(buff, []byte(`,"read_error":true`)...)
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
		node, ok := item.(Node)
		if !ok {
			return fmt.Errorf("encoding %s: not an item of an analyzed tree", item.GetPath())
		}
		if err := node.EncodeJSON(writer, false, attributes); err != nil {
			return err
		}
	}

	if _, err := writer.Write([]byte("]")); err != nil {
		return err
	}
	return nil
}

// EncodeJSON writes JSON representation of file
func (f *File) EncodeJSON(writer io.Writer, _ bool, attributes fs.JSONAttributes) error {
	buff := make([]byte, 0, 20)

	buff = append(buff, []byte(`{"name":`)...)
	if err := addString(&buff, f.GetName()); err != nil {
		return err
	}
	if attributes.Includes("asize") && f.GetSize() > 0 {
		buff = append(buff, []byte(`,"asize":`)...)
		buff = append(buff, []byte(strconv.FormatInt(f.GetSize(), 10))...)
	}
	if attributes.Includes("dsize") && f.GetUsage() > 0 {
		buff = append(buff, []byte(`,"dsize":`)...)
		buff = append(buff, []byte(strconv.FormatInt(f.GetUsage(), 10))...)
	}
	if attributes.Includes("shsize") && f.SharedUsage > 0 {
		buff = append(buff, []byte(`,"shsize":`)...)
		buff = append(buff, []byte(strconv.FormatInt(f.SharedUsage, 10))...)
	}
	if attributes.Includes("mtime") && !f.GetMtime().IsZero() {
		buff = append(buff, []byte(`,"mtime":`)...)
		buff = append(buff, []byte(strconv.FormatInt(f.GetMtime().Unix(), 10))...)
	}

	if attributes.Includes("notreg") && f.Flag == '@' {
		buff = append(buff, []byte(`,"notreg":true`)...)
	}
	if attributes.Includes("ino") && f.Flag == 'H' {
		buff = append(buff, []byte(`,"ino":`+strconv.FormatUint(f.Mli, 10)+`,"hlnkc":true`)...)
	}

	buff = append(buff, '}')

	if _, err := writer.Write(buff); err != nil {
		return err
	}
	return nil
}

// appendDirSizes appends the sizes of a directory as ncdu reads them, and its
// totals as gdu reads them back.
//
// ncdu takes asize and dsize for what the directory itself takes, and adds up
// the entries for the totals. dsize is so the disk usage of the directory
// itself, ownUsage; asize is left out, as gdu does not count an apparent size
// of a directory. The totals, which keep a directory without its entries (a
// depth-limited export) whole on import, go under keys ncdu ignores:
// gdu_asize, gdu_dsize and items. Each is written when its attribute is
// selected: asize, dsize and items.
func appendDirSizes(buff []byte, attributes fs.JSONAttributes, ownUsage, size, usage, itemCount int64) []byte {
	if attributes.Includes("dsize") && ownUsage > 0 {
		buff = append(buff, `,"dsize":`...)
		buff = strconv.AppendInt(buff, ownUsage, 10)
	}
	if attributes.Includes("asize") {
		buff = append(buff, `,"`+totalSizeKey+`":`...)
		buff = strconv.AppendInt(buff, size, 10)
	}
	if attributes.Includes("dsize") {
		buff = append(buff, `,"`+totalUsageKey+`":`...)
		buff = strconv.AppendInt(buff, usage, 10)
	}
	if attributes.Includes("items") {
		buff = append(buff, `,"items":`...)
		buff = strconv.AppendInt(buff, itemCount, 10)
	}
	return buff
}

// The keys of the totals of a directory in an export, see appendDirSizes.
const (
	totalSizeKey  = "gdu_asize"
	totalUsageKey = "gdu_dsize"
)

func addString(buff *[]byte, val string) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	*buff = append(*buff, b...)
	return err
}
