package ioc

var ConController Container = &MapContainer{
	name:    "containerMap",
	storage: make(map[string]Object),
}

var Api Container = &MapContainer{
	name:    "apiContainer",
	storage: make(map[string]Object),
}
