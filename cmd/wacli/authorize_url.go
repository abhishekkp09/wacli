package main

import (
	"fmt"
	"os"

	"github.com/abhishekkp09/wacli/internal/out"
	"github.com/spf13/cobra"
)

// authorizeURL / authorizeMessage mirror the hatch-wai-cli `authorize-url`
// command's static payload. This is a purely cosmetic, offline command: it
// prints the Hatch companion onboarding link and never touches the store or the
// WhatsApp connection. The actual session is established by `wacli auth`.
const (
	authorizeURL     = "https://hatch.ecto1.ai/connect/whatsapp_companion"
	authorizeMessage = "Open the connect_url in Hatch to connect WhatsApp Companion."
)

func newAuthorizeURLCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "authorize-url",
		Short: "Return the Hatch popup URL for WhatsApp companion onboarding.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags != nil && flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]string{
					"connect_url": authorizeURL,
					"message":     authorizeMessage,
				})
			}
			if _, err := fmt.Fprintln(os.Stdout, authorizeURL); err != nil {
				return err
			}
			_, err := fmt.Fprintln(os.Stdout, authorizeMessage)
			return err
		},
	}
}
