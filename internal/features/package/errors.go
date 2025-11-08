package pkg

import (
	"errors"
)

var (
	ErrPackageNotFound   = errors.New("subscription package not found")
	ErrPackageNameTaken  = errors.New("package name already exists")
	ErrPackageOrderTaken = errors.New("package order already exists")
)
