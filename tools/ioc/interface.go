package ioc

type Container interface {
	RegisterContainer(name string, obj Object)
	GetMapContainer(name string) any
	Init() error
}

type Object interface {
	Init() error
}
