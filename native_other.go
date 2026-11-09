//go:build !windows

package main

func runNativeApp(url string) bool { return false }


func runUpdateWindow(url string,ready func(uintptr))bool{return false}
func closeUpdateWindow(){}
