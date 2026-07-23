package tui

// DeleteAction selects how an item is removed from the directory tree.
type DeleteAction uint8

const (
	ActionDelete DeleteAction = iota
	ActionEmpty
	ActionMoveToTrash
)

func (a DeleteAction) Verb() string {
	switch a {
	case ActionEmpty:
		return "empty"
	case ActionMoveToTrash:
		return "move to trash"
	case ActionDelete:
		return "delete"
	}
	return "delete"
}

func (a DeleteAction) Acting() string {
	switch a {
	case ActionEmpty:
		return "emptying"
	case ActionMoveToTrash:
		return "moving to trash"
	case ActionDelete:
		return "deleting"
	}
	return "deleting"
}
