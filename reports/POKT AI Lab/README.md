# Pocket Network Reports

This folder contains all the reports originated from the [Pocket Network grant](https://forum.pokt.network/t/open-pokt-ai-lab-socket/5056).
A shorter version of these reports can be found in the [Karma GAP](https://gap.karmahq.xyz/project/poktscan---pnyx/grants?tab=overview&grantId=0x910b75daac071c3957b9398c37c9ec8cb82297047a96f8ce5b116673d275402d) app, used for tracking by the Pocket Network DAO. As expected by a DAO grant, we provide all the files (and versioning) of the reports as they are written.

## Contributing / Building

To reproduce or contributing to these documents use Pull Requests as for any other part of the repository. If you are using VS Code's `LaTeX-Workshop`  extension, then the configuration to be added in the `settings.json` file is the following:

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