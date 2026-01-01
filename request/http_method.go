package request

type httpMethod string

const HTTPMethod httpMethod = ""

func (h httpMethod) String() string {
	return string(h)
}

func (h httpMethod) Get() string {
	return "GET"
}

func (h httpMethod) Post() string {
	return "POST"
}

func (h httpMethod) Put() string {
	return "PUT"
}

func (h httpMethod) Delete() string {
	return "DELETE"
}

func (h httpMethod) Patch() string {
	return "PATCH"
}

func (h httpMethod) Head() string {
	return "HEAD"
}

func (h httpMethod) Options() string {
	return "OPTIONS"
}
