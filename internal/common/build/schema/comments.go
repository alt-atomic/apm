// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package schema

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// StructDocKey специальный ключ для doc comment структуры
const StructDocKey = "_doc"

// CommentParser парсер комментариев из Go исходников
type CommentParser struct {
	// структура -> поле -> комментарий
	// для doc comment структуры используется ключ StructDocKey ("_doc")
	comments map[string]map[string]string
}

// NewCommentParser создаёт новый парсер комментариев
func NewCommentParser() *CommentParser {
	return &CommentParser{
		comments: make(map[string]map[string]string),
	}
}

// ParseSource парсит Go код из строки
func (p *CommentParser) ParseSource(source string) error {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, "source.go", source, parser.ParseComments)
	if err != nil {
		return err
	}

	p.parseFile(file)
	return nil
}

// parseFile обрабатывает AST файла
func (p *CommentParser) parseFile(file *ast.File) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}

			structName := ts.Name.Name
			if p.comments[structName] == nil {
				p.comments[structName] = make(map[string]string)
			}

			// Парсим doc comment структуры
			// Приоритет: комментарий у TypeSpec, затем у GenDecl
			if ts.Doc != nil {
				p.comments[structName][StructDocKey] = strings.TrimSpace(ts.Doc.Text())
			} else if genDecl.Doc != nil {
				p.comments[structName][StructDocKey] = strings.TrimSpace(genDecl.Doc.Text())
			}

			// Парсим комментарии полей
			for _, field := range st.Fields.List {
				if len(field.Names) == 0 {
					continue
				}

				fieldName := field.Names[0].Name

				// Ищем комментарий перед полем
				if field.Doc != nil {
					comment := strings.TrimSpace(field.Doc.Text())
					p.comments[structName][fieldName] = comment
				}
			}
		}
	}
}

// GetAllComments возвращает все комментарии
func (p *CommentParser) GetAllComments() map[string]map[string]string {
	return p.comments
}
