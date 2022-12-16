package utils

// PtrToStr converts a pointer to a string. nil pointer returns an empty string
func PtrToStr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// StrToPtr returns a pointer from a string value
func StrToPtr(str string) *string {
	return &str
}
