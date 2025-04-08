package project

//
//import "unicode"
//
//// ValidateCmdName returns source without any dashes and underscore.
//// If there will be dash or underscore, next letter will be uppered.
//// It supports only ASCII (1-byte character) strings.
//// https://github.com/spf13/cobra/issues/269
//func ValidateCmdName(source string) string {
//	i := 0
//	l := len(source)
//	// The output is initialized on demand, then first dash or underscore
//	// occurs.
//	var output string
//
//	for i < l {
//		if source[i] == '-' || source[i] == '_' {
//			if output == "" {
//				output = source[:i]
//			}
//
//			// If it's last rune, and it's dash or underscore,
//			// don't add it output and break the loop.
//			if i == l-1 {
//				break
//			}
//
//			// If next character is dash or underscore,
//			// just skip the current character.
//			if source[i+1] == '-' || source[i+1] == '_' {
//				i++
//				continue
//			}
//
//			// If the current character is dash or underscore,
//			// upper next letter and add to output.
//			output += string(unicode.ToUpper(rune(source[i+1])))
//			// We know, what source[i] is dash or underscore and source[i+1] is
//			// upper character, so make i = i+2.
//			i += 2
//			continue
//		}
//
//		// If the current character isn't dash or underscore,
//		// just add it.
//		if output != "" {
//			output += string(source[i])
//		}
//		i++
//	}
//
//	if output == "" {
//		return source // source is initially valid name.
//	}
//	return output
//}
