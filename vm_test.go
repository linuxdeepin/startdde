// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetProductType(t *testing.T) {
	t.Run("Test get product type", func(t *testing.T) {
		productType := getProductType()
		assert.Contains(t, []string{"", "Desktop", "Server"}, productType)
	})
}

func TestIsInVM(t *testing.T) {
	t.Run("Test is in VM", func(t *testing.T) {
		_, err := isInVM()
		assert.NoError(t, err)
	})
}
