package main

import (
	"fmt"
	"log"
	"os"

	"codex-service/internal/auth"
	"codex-service/internal/config"
	"codex-service/internal/proxy"
	"codex-service/internal/server"
)

func main() {
	cfg := config.Load()
	store := auth.NewStore(cfg.DataDir)

	// Handle subcommands
	if len(os.Args) > 1 && os.Args[1] == "login" {
		if err := doLogin(store); err != nil {
			log.Fatalf("登入失敗: %v", err)
		}
		return
	}

	// Load or obtain credentials
	creds, err := store.Load()
	if err != nil {
		fmt.Println("尚未登入，開始認證流程...")
		if err := doLogin(store); err != nil {
			log.Fatalf("登入失敗: %v", err)
		}
		creds, err = store.Load()
		if err != nil {
			log.Fatalf("無法載入憑證: %v", err)
		}
	}

	tm := auth.NewTokenManager(creds, store)

	// Verify token is usable (attempt refresh if needed)
	if _, _, err := tm.GetToken(); err != nil {
		fmt.Println("Token 已失效，重新認證...")
		if err := doLogin(store); err != nil {
			log.Fatalf("登入失敗: %v", err)
		}
		creds, _ = store.Load()
		tm.UpdateCredentials(creds)
	}

	handler := proxy.NewHandler(tm)
	srv := server.New(handler, cfg.ListenAddr, cfg.LocalAuth)

	if err := srv.Start(); err != nil {
		log.Fatalf("伺服器錯誤: %v", err)
	}
}

func doLogin(store *auth.Store) error {
	creds, err := auth.DeviceCodeLogin()
	if err != nil {
		return err
	}
	if err := store.Save(creds); err != nil {
		return fmt.Errorf("儲存憑證失敗: %w", err)
	}
	fmt.Println("登入成功！憑證已儲存。")
	return nil
}
