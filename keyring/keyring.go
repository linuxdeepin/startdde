// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package keyring

// #cgo CFLAGS: -W -Wall -DSECRET_API_SUBJECT_TO_CHANGE -fstack-protector-all -fPIC
// #cgo pkg-config: libsecret-unstable
// #include "keyring.h"
import "C"

import (
	"fmt"
)

// CheckLogin check whether the login keyring exists.
// If not, create it and set as default.
func CheckLogin() error {
	r := C.check_login()
	if r != 0 {
		return fmt.Errorf("failed to check login keyring")
	}
	return nil
}
