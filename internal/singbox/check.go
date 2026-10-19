package singbox
import("context";"errors";"os/exec")
type ExternalChecker struct{Binary string}
func(c ExternalChecker)Check(ctx context.Context,path string)error{if exec.CommandContext(ctx,c.Binary,"check","-c",path).Run()!=nil{return errors.New("sing-box validation failed")};return nil}
