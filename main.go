package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	// 检查命令行参数长度
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// 处理子命令
	subcmd := os.Args[1]
	switch subcmd {
	case "new":
		// 处理 new 子命令
		handleNewCommand()
	case "proto":
		// 处理 proto 子命令
		handleProtoCommand()
	default:
		fmt.Printf("Unknown command: %s\n", subcmd)
		printUsage()
		os.Exit(1)
	}
}

// handleNewCommand 处理 new 子命令
func handleNewCommand() {
	// 手动解析命令行参数，支持标志在位置参数之后
	nomod := false
	repoURL := "https://github.com/sunmery/connect-example-fast.git"
	var args []string

	for i := 0; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--nomod":
			nomod = true
		case "-r":
			i++
			if i < len(os.Args) {
				repoURL = os.Args[i]
			}
		default:
			args = append(args, arg)
		}
	}

	// 处理位置参数
	if len(args) < 3 || args[1] != "new" {
		printUsage()
		os.Exit(1)
	}

	appPath := args[2]
	parts := strings.Split(appPath, "/")
	appName := parts[len(parts)-1]

	// 模板仓库URL（使用用户指定或默认值）
	templateURL := repoURL
	targetPath := filepath.Join(".", appPath)

	// 从远程仓库克隆代码
	if err := gitClone(templateURL, targetPath); err != nil {
		fmt.Printf("Failed to clone template: %v\n", err)
		os.Exit(1)
	}

	// 根据--nomod参数执行不同的逻辑
	if nomod {
		// 大仓模式：计算完整的模块路径，包括当前目录名称
		currentDir, err := os.Getwd()
		if err != nil {
			fmt.Printf("Failed to get current directory: %v\n", err)
			os.Exit(1)
		}
		rootDirName := filepath.Base(currentDir)
		fullModulePath := rootDirName + "/" + appPath

		// 大仓模式
		if err := handleMonorepoMode(targetPath, fullModulePath, appName); err != nil {
			fmt.Printf("Failed to handle monorepo mode: %v\n", err)
			os.Exit(1)
		}
	} else {
		// 普通模式
		// 修改go.mod文件
		goModPath := filepath.Join(targetPath, "go.mod")
		if err := updateGoMod(goModPath, "connect-go-example", appName); err != nil {
			fmt.Printf("Failed to update go.mod: %v\n", err)
			os.Exit(1)
		}

		// 重命名cmd/server目录为cmd/<appName>
		oldCmdPath := filepath.Join(targetPath, "cmd", "server")
		newCmdPath := filepath.Join(targetPath, "cmd", appName)
		if err := os.Rename(oldCmdPath, newCmdPath); err != nil {
			fmt.Printf("Failed to rename cmd directory: %v\n", err)
			os.Exit(1)
		}

		// 修改所有go文件中的import路径
		if err := updateAllGoFiles(targetPath, "connect-go-example", appName); err != nil {
			fmt.Printf("Failed to update go files: %v\n", err)
			os.Exit(1)
		}

		// 修改所有proto文件中的package和go_package字段
		if err := updateProtoFiles(targetPath, "connect-go-example", appName); err != nil {
			fmt.Printf("Failed to update proto files: %v\n", err)
			os.Exit(1)
		}

		// 确保main.go中有必要的import
		mainFilePath := filepath.Join(newCmdPath, "main.go")
		if err := ensureMainImports(mainFilePath, appName); err != nil {
			fmt.Printf("Failed to update main.go imports: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Application %s created successfully at %s\n", appName, targetPath)
}

// handleProtoCommand 处理 proto 子命令
func handleProtoCommand() {
	if len(os.Args) < 3 {
		printProtoUsage()
		os.Exit(1)
	}

	protoSubcmd := os.Args[2]
	switch protoSubcmd {
	case "add":
		// 处理 proto add 子命令
		if len(os.Args) < 4 {
			fmt.Println("Usage: co proto add <proto-path>")
			os.Exit(1)
		}
		protoPath := os.Args[3]
		if err := addProtoFile(protoPath); err != nil {
			fmt.Printf("Failed to add proto file: %v\n", err)
			os.Exit(1)
		}

	case "server":
		// 处理 proto server 子命令
		targetDir := "internal/service"
		if len(os.Args) > 5 && os.Args[4] == "-t" {
			targetDir = os.Args[5]
		}
		if len(os.Args) < 4 {
			fmt.Println("Usage: co proto server <proto-path> [-t <target-dir>]")
			os.Exit(1)
		}
		protoPath := os.Args[3]
		if err := generateProtoServer(protoPath, targetDir); err != nil {
			fmt.Printf("Failed to generate proto server: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown proto command: %s\n", protoSubcmd)
		printProtoUsage()
		os.Exit(1)
	}
}

// printUsage 打印使用帮助
func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  co new <application/path> [-r <repo-url>] [--nomod]")
	fmt.Println("  co proto [add|client|server] [options]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  new       Create a new application from template")
	fmt.Println("  proto     Proto file generation commands")
	fmt.Println()
	fmt.Println("Proto Subcommands:")
	printProtoUsage()
}

// printProtoUsage 打印 proto 子命令使用帮助
func printProtoUsage() {
	fmt.Println("  proto add <proto-path>        Add a new proto file")
	fmt.Println("  proto client <proto-path>     Generate proto client codes")
	fmt.Println("  proto server <proto-path>     Generate proto server codes")
	fmt.Println("    -t <target-dir>            Target directory for server codes (default: internal/service)")
}

// addProtoFile 添加新的proto文件
func addProtoFile(protoPath string) error {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(protoPath), 0755); err != nil {
		return err
	}

	// 生成proto文件内容
	protoContent := generateProtoContent(protoPath)

	// 写入文件
	return os.WriteFile(protoPath, []byte(protoContent), 0644)
}

// generateProtoContent 生成proto文件内容
func generateProtoContent(protoPath string) string {
	// 从路径中提取服务名称和包名
	// 例如: api/helloworld/demo.proto -> helloworld, demo
	pathParts := strings.Split(protoPath, "/")
	if len(pathParts) < 3 {
		return "" // 无效路径
	}

	// 提取包名和服务名
	pkgName := pathParts[len(pathParts)-2]
	serviceName := strings.TrimSuffix(pathParts[len(pathParts)-1], ".proto")

	// 生成go_package
	goPkg := fmt.Sprintf("%s/%s/%s/%s;%s", pathParts[len(pathParts)-3], pkgName, serviceName, serviceName, serviceName)

	// 生成proto文件内容
	return fmt.Sprintf(`syntax = "proto3";

package %s;

option go_package = "%s";
option java_multiple_files = true;
option java_package = "%s";

service %s {
    rpc Create%s (Create%sRequest) returns (Create%sReply);
    rpc Update%s (Update%sRequest) returns (Update%sReply);
    rpc Delete%s (Delete%sRequest) returns (Delete%sReply);
    rpc Get%s (Get%sRequest) returns (Get%sReply);
    rpc List%s (List%sRequest) returns (List%sReply);
}

message Create%sRequest {}
message Create%sReply {}

message Update%sRequest {}
message Update%sReply {}

message Delete%sRequest {}
message Delete%sReply {}

message Get%sRequest {}
message Get%sReply {}

message List%sRequest {}
message List%sReply {}
`,
		pkgName,
		goPkg,
		pkgName,
		strings.Title(serviceName),
		strings.Title(serviceName), strings.Title(serviceName), strings.Title(serviceName),
		strings.Title(serviceName), strings.Title(serviceName), strings.Title(serviceName),
		strings.Title(serviceName), strings.Title(serviceName), strings.Title(serviceName),
		strings.Title(serviceName), strings.Title(serviceName), strings.Title(serviceName),
		strings.Title(serviceName), strings.Title(serviceName), strings.Title(serviceName),
		strings.Title(serviceName), strings.Title(serviceName),
		strings.Title(serviceName), strings.Title(serviceName),
		strings.Title(serviceName), strings.Title(serviceName),
		strings.Title(serviceName), strings.Title(serviceName),
		strings.Title(serviceName), strings.Title(serviceName),
	)
}

// generateProtoServer 生成proto服务器代码
func generateProtoServer(protoPath, targetDir string) error {
	// 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	// 从proto路径中提取服务名称
	serviceName := strings.TrimSuffix(filepath.Base(protoPath), ".proto")
	serviceName = strings.Title(serviceName)

	// 生成服务代码
	serverCode := generateServerCode(protoPath, serviceName)

	// 写入文件
	targetFile := filepath.Join(targetDir, strings.ToLower(serviceName)+"_service.go")
	return os.WriteFile(targetFile, []byte(serverCode), 0644)
}

// generateServerCode 生成connect-go风格的服务器代码
func generateServerCode(protoPath, serviceName string) string {
	// 从proto路径中提取包名和服务信息
	pathParts := strings.Split(protoPath, "/")
	if len(pathParts) < 3 {
		return "" // 无效路径
	}

	// 提取包名和服务名
	pkgName := pathParts[len(pathParts)-2]
	serviceNameLower := strings.ToLower(serviceName)
	pbPkg := fmt.Sprintf("%s/%s/%s", pathParts[len(pathParts)-3], pkgName, serviceNameLower)
	connectPkg := fmt.Sprintf("%s/%s/%s/%sconnect", pathParts[len(pathParts)-3], pkgName, serviceNameLower, serviceNameLower)

	// 生成服务代码，只包含必要部分
	return fmt.Sprintf(`package service

import (
    "context"

    pb "%s"
    %sconnect "%s"

    "connectrpc.com/connect"
    "%s/internal/biz"
)

// %sService 实现 Connect 服务
type %sService struct {
    // 业务逻辑依赖
    uc *biz.%sUseCase
}

// 显式接口检查
var _ %sconnect.%sServiceHandler = (*%sService)(nil)
`, 
		pbPkg, serviceNameLower, connectPkg, pathParts[len(pathParts)-3],
		serviceName, serviceName, serviceName,
		serviceNameLower, serviceName, serviceName,
	)
}

// gitClone 从远程仓库克隆代码
func gitClone(url, path string) error {
	// 确保目标目录不存在
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return fmt.Errorf("target directory %s already exists", path)
	}

	// 创建父目录
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// 执行git clone命令
	cmd := exec.Command("git", "clone", url, path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// updateGoMod 更新go.mod文件中的module名称
func updateGoMod(path, oldModule, newModule string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	newData := strings.Replace(string(data), "module "+oldModule, "module "+newModule, 1)
	return os.WriteFile(path, []byte(newData), 0644)
}

// updateAllGoFiles 更新所有go文件中的import路径
func updateAllGoFiles(root, oldModule, newModule string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".go" {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			newData := strings.ReplaceAll(string(data), oldModule, newModule)
			if err := os.WriteFile(path, []byte(newData), 0644); err != nil {
				return err
			}
		}

		return nil
	})
}

// updateProtoFiles 更新所有proto文件中的package和go_package字段
func updateProtoFiles(root, oldModule, newModule string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".proto" {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			content := string(data)

			// 修改go_package中的旧module名称为新名称
			content = strings.ReplaceAll(content, oldModule, newModule)

			// 修改package字段，将第一个部分替换为newModule
			packageRegex := regexp.MustCompile(`package\s+(\w+)\.(v\d+);`)
			content = packageRegex.ReplaceAllString(content, "package "+newModule+".$2;")

			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return err
			}
		}

		return nil
	})
}

// handleMonorepoMode 处理大仓模式的逻辑
func handleMonorepoMode(targetPath, fullModulePath, appName string) error {
	fmt.Printf("Entering monorepo mode for %s with module path %s\n", targetPath, fullModulePath)

	// 1. 重命名cmd/server目录为cmd/<appName>
	oldCmdPath := filepath.Join(targetPath, "cmd", "server")
	newCmdPath := filepath.Join(targetPath, "cmd", appName)
	if err := os.Rename(oldCmdPath, newCmdPath); err != nil {
		return fmt.Errorf("failed to rename cmd directory: %w", err)
	}
	fmt.Printf("Renamed cmd/server to cmd/%s\n", appName)

	// 2. 删除生成的go.mod和go.sum文件
	goModPath := filepath.Join(targetPath, "go.mod")
	if err := os.Remove(goModPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove go.mod: %w", err)
		}
	}
	fmt.Printf("Removed go.mod file\n")

	goSumPath := filepath.Join(targetPath, "go.sum")
	if err := os.Remove(goSumPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove go.sum: %w", err)
		}
	}
	fmt.Printf("Removed go.sum file\n")

	// 3. 删除生成的api目录
	apiPath := filepath.Join(targetPath, "api")
	if err := os.RemoveAll(apiPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove api directory: %w", err)
		}
	}
	fmt.Printf("Removed api directory\n")

	// 4. 修改所有go文件中的import路径，使用完整路径
	if err := updateGoFilesForMonorepo(targetPath, "connect-go-example", fullModulePath); err != nil {
		return fmt.Errorf("failed to update go files: %w", err)
	}
	fmt.Printf("Updated import paths in go files\n")

	// 5. 确保main.go中有必要的import
	mainFilePath := filepath.Join(newCmdPath, "main.go")
	if err := ensureMainImports(mainFilePath, appName); err != nil {
		return fmt.Errorf("failed to update main.go imports: %w", err)
	}
	fmt.Printf("Ensured main.go imports\n")

	return nil
}

// updateGoFilesForMonorepo 更新大仓模式下的go文件import路径
func updateGoFilesForMonorepo(root, oldModule, newModulePath string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".go" {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			content := string(data)

			// 解析模块路径，获取根目录名称
			parts := strings.Split(newModulePath, "/")
			if len(parts) < 2 {
				return fmt.Errorf("invalid module path: %s", newModulePath)
			}
			rootDirName := parts[0]

			// 替换普通导入路径
			content = strings.ReplaceAll(content, oldModule, newModulePath)

			// 特殊处理api导入路径，使用根目录的api目录
			// 将 newModulePath + "/api/" 替换为 rootDirName + "/api/"
			apiPattern := newModulePath + "/api/"
			apiReplacement := rootDirName + "/api/"
			content = strings.ReplaceAll(content, apiPattern, apiReplacement)

			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return err
			}
		}

		return nil
	})
}

// ensureMainImports 确保main.go中有必要的import
func ensureMainImports(path, appName string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	content := string(data)

	// 检查是否已经有flag和os import
	if !strings.Contains(content, `"flag"`) || !strings.Contains(content, `"os"`) {
		// 找到import块并添加flag和os
		importRegex := regexp.MustCompile(`import \(([^)]+)\)`)
		matches := importRegex.FindStringSubmatch(content)
		if len(matches) < 2 {
			return fmt.Errorf("could not find import block in main.go")
		}

		importBlock := matches[1]
		newImportBlock := importBlock

		// 添加flag包
		if !strings.Contains(importBlock, `"flag"`) {
			newImportBlock += "\n\t\"flag\""
		}
		// 添加os包
		if !strings.Contains(importBlock, `"os"`) {
			newImportBlock += "\n\t\"os\""
		}

		content = importRegex.ReplaceAllString(content, "import ("+newImportBlock+")")
	}

	return os.WriteFile(path, []byte(content), 0644)
}
