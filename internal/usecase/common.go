package usecase

import "strconv"

func idToString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
