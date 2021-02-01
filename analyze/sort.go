package analyze

func (f Files) Len() int           { return len(f) }
func (f Files) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f Files) Less(i, j int) bool { return f[i].Usage > f[j].Usage }

// ByApparentSize sorts files by apparent size
type ByApparentSize Files

func (f ByApparentSize) Len() int           { return len(f) }
func (f ByApparentSize) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByApparentSize) Less(i, j int) bool { return f[i].Size > f[j].Size }

// ByItemCount sorts files by item count
type ByItemCount Files

func (f ByItemCount) Len() int           { return len(f) }
func (f ByItemCount) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByItemCount) Less(i, j int) bool { return f[i].ItemCount > f[j].ItemCount }

// ByName sorts files by name
type ByName Files

func (f ByName) Len() int           { return len(f) }
func (f ByName) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByName) Less(i, j int) bool { return f[i].Name > f[j].Name }
