package cloudprovider

// TODO: CreateWindowsVM should return vendor session for future interaction with created instance
type Cloud interface {
	CreateWindowsVM(imageId, instanceType, keyName *string) error
	DestroyWindowsVM() error
}
