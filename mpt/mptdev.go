package mpt

import (
	"blockchain/common"
)

type MPT interface {
	Root() common.Hash
}
