package generator

type Command struct {
	CmdName   string
	CmdParent string
	*Project
}
