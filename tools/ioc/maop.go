package ioc

type MapContainer struct {
	name    string
	storage map[string]Object
}

func (m *MapContainer) RegisterContainer(name string, obj Object) {
	m.storage[name] = obj
}

func (m *MapContainer) GetMapContainer(name string) any {
	obj, ok := m.storage[name]
	if !ok {
		return nil
	}
	return obj
}

func (m *MapContainer) Init() error {
	for _, obj := range m.storage {
		if err := obj.Init(); err != nil {
			return err
		}
	}
	return nil
}
