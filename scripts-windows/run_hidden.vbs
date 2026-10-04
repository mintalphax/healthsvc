' Run a command hidden (no visible window)
' Usage: wscript //B //Nologo run_hidden.vbs "command"

If WScript.Arguments.Count > 0 Then
    Set shell = CreateObject("WScript.Shell")
    shell.Run WScript.Arguments(0), 0, False
End If
