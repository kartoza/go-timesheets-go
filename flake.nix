{
  description = "Development environment for Kartoza Timesheet App";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        
        # Custom Neovim configuration
        neovimConfig = pkgs.neovim.override {
          configure = {
            customRC = ''
              " Basic settings
              set number
              set relativenumber
              set tabstop=4
              set shiftwidth=4
              set expandtab
              set autoindent
              set smartindent
              set mouse=a
              set clipboard=unnamedplus
              set ignorecase
              set smartcase
              set incsearch
              set hlsearch
              set wrap
              set linebreak
              set scrolloff=8
              set signcolumn=yes
              set updatetime=300
              set timeoutlen=500
              set completeopt=menu,menuone,noselect
              
              " Color scheme
              set termguicolors
              colorscheme habamax
              
              " Key mappings
              let mapleader = " "
              nnoremap <leader>ff <cmd>Telescope find_files<cr>
              nnoremap <leader>fg <cmd>Telescope live_grep<cr>
              nnoremap <leader>fb <cmd>Telescope buffers<cr>
              nnoremap <leader>fh <cmd>Telescope help_tags<cr>
              nnoremap <leader>e <cmd>NvimTreeToggle<cr>
              
              " Go-specific mappings
              nnoremap <leader>gr <cmd>GoRun<cr>
              nnoremap <leader>gt <cmd>GoTest<cr>
              nnoremap <leader>gb <cmd>GoBuild<cr>
              nnoremap <leader>gf <cmd>GoFmt<cr>
              nnoremap <leader>gi <cmd>GoImports<cr>
              
              " LSP mappings
              nnoremap <leader>gd <cmd>lua vim.lsp.buf.definition()<cr>
              nnoremap <leader>gD <cmd>lua vim.lsp.buf.declaration()<cr>
              nnoremap <leader>gr <cmd>lua vim.lsp.buf.references()<cr>
              nnoremap <leader>gi <cmd>lua vim.lsp.buf.implementation()<cr>
              nnoremap <leader>K <cmd>lua vim.lsp.buf.hover()<cr>
              nnoremap <leader>rn <cmd>lua vim.lsp.buf.rename()<cr>
              nnoremap <leader>ca <cmd>lua vim.lsp.buf.code_action()<cr>
              nnoremap <leader>f <cmd>lua vim.lsp.buf.format()<cr>
              
              " Diagnostic mappings
              nnoremap <leader>dd <cmd>lua vim.diagnostic.open_float()<cr>
              nnoremap <leader>dn <cmd>lua vim.diagnostic.goto_next()<cr>
              nnoremap <leader>dp <cmd>lua vim.diagnostic.goto_prev()<cr>
              
              " Copilot mappings
              imap <silent><script><expr> <C-J> copilot#Accept("\<CR>")
              let g:copilot_no_tab_map = v:true
              imap <C-]> <Plug>(copilot-next)
              imap <C-[> <Plug>(copilot-previous)
              imap <C-\> <Plug>(copilot-suggest)
              
              " Auto-commands
              autocmd BufWritePre *.go lua vim.lsp.buf.format()
              autocmd BufWritePre *.go lua goimports(1000)
              
              " Lua configuration
              lua << EOF
              -- LSP Setup
              local lspconfig = require('lspconfig')
              local capabilities = require('cmp_nvim_lsp').default_capabilities()
              
              -- Go LSP
              lspconfig.gopls.setup{
                capabilities = capabilities,
                settings = {
                  gopls = {
                    analyses = {
                      unusedparams = true,
                    },
                    staticcheck = true,
                    gofumpt = true,
                  },
                },
              }
              
              -- Completion setup
              local cmp = require'cmp'
              cmp.setup({
                snippet = {
                  expand = function(args)
                    require('luasnip').lsp_expand(args.body)
                  end,
                },
                mapping = cmp.mapping.preset.insert({
                  ['<C-b>'] = cmp.mapping.scroll_docs(-4),
                  ['<C-f>'] = cmp.mapping.scroll_docs(4),
                  ['<C-Space>'] = cmp.mapping.complete(),
                  ['<C-e>'] = cmp.mapping.abort(),
                  ['<CR>'] = cmp.mapping.confirm({ select = true }),
                  ['<Tab>'] = cmp.mapping(function(fallback)
                    if cmp.visible() then
                      cmp.select_next_item()
                    else
                      fallback()
                    end
                  end, { 'i', 's' }),
                  ['<S-Tab>'] = cmp.mapping(function(fallback)
                    if cmp.visible() then
                      cmp.select_prev_item()
                    else
                      fallback()
                    end
                  end, { 'i', 's' }),
                }),
                sources = cmp.config.sources({
                  { name = 'nvim_lsp' },
                  { name = 'luasnip' },
                  { name = 'copilot' },
                }, {
                  { name = 'buffer' },
                })
              })
              
              -- Treesitter setup
              require'nvim-treesitter.configs'.setup {
                ensure_installed = { "go", "lua", "vim", "vimdoc", "query", "markdown", "json", "yaml", "toml", "bash" },
                sync_install = false,
                auto_install = true,
                highlight = {
                  enable = true,
                  additional_vim_regex_highlighting = false,
                },
                indent = {
                  enable = true,
                },
                incremental_selection = {
                  enable = true,
                  keymaps = {
                    init_selection = "gnn",
                    node_incremental = "grn",
                    scope_incremental = "grc",
                    node_decremental = "grm",
                  },
                },
              }
              
              -- Telescope setup
              require('telescope').setup{
                defaults = {
                  file_ignore_patterns = { "node_modules", ".git/", "dist/", "build/" },
                },
                pickers = {
                  find_files = {
                    hidden = true,
                  },
                },
              }
              
              -- Nvim-tree setup
              require("nvim-tree").setup({
                sort_by = "case_sensitive",
                view = {
                  width = 30,
                },
                renderer = {
                  group_empty = true,
                },
                filters = {
                  dotfiles = false,
                },
              })
              
              -- Function to run goimports
              function goimports(timeout_ms)
                local context = { source = { organizeImports = true } }
                vim.validate { context = { context, "table", true } }
                local params = vim.lsp.util.make_range_params()
                params.context = context
                local result = vim.lsp.buf_request_sync(0, "textDocument/codeAction", params, timeout_ms)
                if not result or next(result) == nil then
                  return
                end
                local actions = result[1].result
                if not actions then
                  return
                end
                local action = actions[1]
                if action.edit or type(action.command) == "table" then
                  if action.edit then
                    vim.lsp.util.apply_workspace_edit(action.edit)
                  end
                  if type(action.command) == "table" then
                    vim.lsp.buf.execute_command(action.command)
                  end
                else
                  vim.lsp.buf.execute_command(action)
                end
              end
              EOF
            '';
            
            packages.myVimPackage = with pkgs.vimPlugins; {
              start = [
                # LSP and completion
                nvim-lspconfig
                nvim-cmp
                cmp-nvim-lsp
                cmp-buffer
                cmp-path
                cmp-cmdline
                luasnip
                cmp_luasnip
                
                # Treesitter
                nvim-treesitter.withAllGrammars
                nvim-treesitter-textobjects
                
                # File explorer and fuzzy finder
                nvim-tree-lua
                telescope-nvim
                plenary-nvim
                nvim-web-devicons
                
                # Go development
                vim-go
                
                # Git integration
                gitsigns-nvim
                fugitive
                
                # Copilot
                copilot-vim
                
                # UI enhancements
                lualine-nvim
                bufferline-nvim
                indent-blankline-nvim
                
                # Colorschemes
                gruvbox-nvim
                catppuccin-nvim
                tokyonight-nvim
                
                # Utility
                comment-nvim
                autopairs
                which-key-nvim
                
                # Markdown
                markdown-preview-nvim
              ];
            };
          };
        };
        
        # Development tools
        devTools = with pkgs; [
          # Go ecosystem
          go
          gopls
          gotools
          go-tools
          golangci-lint
          gomodifytags
          gotests
          impl
          reftools
          
          # General development
          git
          gh
          pre-commit
          nodejs_20
          python311
          python311Packages.pip
          
          # Terminal and shell utilities
          zsh
          fish
          starship
          eza
          bat
          fd
          ripgrep
          fzf
          tree
          jq
          yq
          
          # Build tools
          gnumake
          gcc
          pkg-config
          
          # Documentation
          mdbook
          pandoc
          
          # Container tools
          docker
          docker-compose
          
          # Nix tools
          nil # Nix LSP
          nixpkgs-fmt
          nixfmt-classic
          
          # Security tools
          trivy
          
          # System monitoring
          htop
          btop
          
          # Text processing
          gawk
          sed
        ];
        
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = [ neovimConfig ] ++ devTools;
          
          shellHook = ''
            echo "🚀 Kartoza Timesheet App Development Environment"
            echo "=============================================="
            echo "✅ Neovim with Go LSP, Treesitter, and Copilot"
            echo "✅ Go toolchain with gopls and linting tools"
            echo "✅ Modern CLI tools (ripgrep, fd, eza, etc.)"
            echo "✅ Git and GitHub CLI tools"
            echo "✅ Pre-commit hooks ready"
            echo ""
            echo "🔧 Quick commands:"
            echo "  nvim .           # Open Neovim in current directory"
            echo "  go mod init      # Initialize Go module"
            echo "  go get github.com/charmbracelet/bubbletea  # Add Bubbletea"
            echo "  pre-commit install  # Set up quality hooks"
            echo ""
            echo "📚 Neovim key mappings:"
            echo "  <Space>ff        # Find files (Telescope)"
            echo "  <Space>fg        # Live grep (Telescope)"
            echo "  <Space>e         # Toggle file tree"
            echo "  <Space>gr        # Go run"
            echo "  <Space>gt        # Go test"
            echo "  <Space>gd        # Go to definition"
            echo "  <C-J>            # Accept Copilot suggestion"
            echo ""
            echo "🌟 Ubuntu Philosophy: 'I am because we are'"
            echo ""
            
            # Set up environment
            export EDITOR=nvim
            export VISUAL=nvim
            export GOPATH="$HOME/go"
            export PATH="$PATH:$GOPATH/bin"
            
            # Go environment
            export GO111MODULE=on
            export GOPROXY=https://proxy.golang.org,direct
            export GOSUMDB=sum.golang.org
            
            # Enable Copilot
            export COPILOT_NODE_VERSION="18"
            
            # Create workspace directories if they don't exist
            mkdir -p .nvim/undo .nvim/swap .nvim/backup
            
            # Initialize Go module if it doesn't exist
            if [ ! -f "go.mod" ]; then
              echo "💡 Run 'go mod init github.com/timlinux/kartoza-timesheet-app' to initialize Go module"
            fi
            
            # Set up aliases for development
            alias ll='eza -la'
            alias la='eza -la'
            alias ls='eza'
            alias cat='bat'
            alias find='fd'
            alias grep='rg'
            alias vim='nvim'
            alias vi='nvim'
            
            # Go aliases
            alias gor='go run .'
            alias got='go test ./...'
            alias gob='go build .'
            alias gom='go mod tidy'
            alias gof='gofmt -s -w .'
            alias goi='goimports -w .'
            
            # Git aliases
            alias gs='git status'
            alias ga='git add'
            alias gc='git commit'
            alias gp='git push'
            alias gl='git pull'
            alias gd='git diff'
            
            # Check if Bubbletea is in go.mod
            if [ -f "go.mod" ] && grep -q "bubbletea" go.mod; then
              echo "🫧 Bubbletea is ready for TUI development!"
            elif [ -f "go.mod" ]; then
              echo "💡 Add Bubbletea: go get github.com/charmbracelet/bubbletea"
            fi
          '';
          
          # Environment variables
          COPILOT_NODE_VERSION = "18";
          GO111MODULE = "on";
          GOPROXY = "https://proxy.golang.org,direct";
          GOSUMDB = "sum.golang.org";
        };
        
        # Formatter for the flake
        formatter = pkgs.nixpkgs-fmt;
        
        # Apps for easy execution
        apps = {
          nvim = flake-utils.lib.mkApp {
            drv = neovimConfig;
          };
          
          setup = flake-utils.lib.mkApp {
            drv = pkgs.writeShellScriptBin "setup" ''
              echo "Setting up development environment..."
              if [ ! -f "go.mod" ]; then
                go mod init github.com/timlinux/kartoza-timesheet-app
                echo "✅ Go module initialized"
              fi
              
              # Add Bubbletea and common dependencies
              go get github.com/charmbracelet/bubbletea@latest
              go get github.com/charmbracelet/lipgloss@latest
              go get github.com/charmbracelet/bubbles@latest
              
              echo "✅ Bubbletea ecosystem installed"
              echo "🚀 Ready to build amazing TUIs!"
            '';
          };
        };
      }
    );
}