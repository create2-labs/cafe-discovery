//go:build tools

package tools

import (
	_ "github.com/consensys/gnark-crypto/ecc/bn254"
	_ "github.com/ethereum/go-ethereum/ethclient"
	_ "github.com/gofiber/fiber/v2"
	_ "github.com/jackc/pgx/v5"
	_ "golang.org/x/crypto/ssh"
	_ "gorm.io/driver/postgres"
)
