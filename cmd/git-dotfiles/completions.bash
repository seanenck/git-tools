_git_dotfiles() {
  local cur opts chosen offset
  cur=${COMP_WORDS[COMP_CWORD]}
  offset=1
  chosen=${COMP_WORDS[1]}
  if [ -n "$chosen" ]; then
    if [ "$chosen" = "dotfiles" ]; then
      offset=2
    fi
  fi
  if [ "$COMP_CWORD" -eq $offset ]; then
    opts="{{ $.Deploy }} {{ $.Diff }}"
  else
    chosen=${COMP_WORDS[$offset]}
    case "$chosen" in
      "{{ $.Deploy }}")
        opts="{{ $.Args.DryRun }} {{ $.Args.Force }} {{ $.Args.Overwrite }}"
      ;;
      "{{ $.Diff }}")
        opts="{{ $.Args.Verbose }}"
      ;;
    esac
  fi
  if [ -n "$opts" ]; then
    # shellcheck disable=SC2207
    COMPREPLY=($(compgen -W "$opts" -- "$cur"))
  fi
}

complete -F _git_dotfiles -o bashdefault {{ $.Exe }}
