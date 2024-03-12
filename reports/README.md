# Report Latex Template

## Use

### LaTeX-Workshop (VS Code extention)

Add into `settings.json`:

```json
    "latex-workshop.latex.recipes":[
      {
        "name": "pdflatex, biber, pdflatex (2x)",
        "tools": [
          "pdflatex",
          "biber",
          "pdflatex",
          "pdflatex"
        ]
      },

      {
        "name": "pdflatex, biber, makeglossaries, pdflatex (2x)",
        "tools": [
          "pdflatex",
          "biber",
          "makeglossaries",
          "pdflatex",
          "pdflatex"
        ]
      }      
    ],   
    "latex-workshop.latex.tools":[
      {
        "name": "pdflatex",
        "command": "pdflatex",
        "args": [
          "-synctex=1",
          "-interaction=nonstopmode",
          "-file-line-error",
          "%DOC%"
        ]
      },
      {
        "name": "biber",
        "command": "biber",
        "args": [
          "%DOCFILE%"
                ],
        "env": {}
      },             
      {
        "name": "makeglossaries",
        "command": "makeglossaries",
        "args": [
          "%DOCFILE%"
        ]
      }
  ]
```

Then run the recipes `pdflatex, biber, pdflatex (2x)` or `pdflatex, biber, makeglossaries, pdflatex (2x)` on VS code.