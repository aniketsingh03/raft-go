package main

func main() {
	rootCmd := getRootCmd()
	err := rootCmd.Execute()
	if err != nil {
		return
	}
}
