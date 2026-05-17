package interfaces

type IParserStore interface {
	InsertSegmentationConfig(config interface{}) error
	MergeOrCreateNetworkNodes(ctx interface{}, nodes interface{}, hostname string) (interface{}, error)
	AddNetworkGroups(groups interface{}) error
	SaveParsedData(data interface{}) error
	Close() error
}
