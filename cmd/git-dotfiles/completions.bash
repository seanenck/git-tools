_git_dotfiles() {
  local cur opts chosen offset subset
  cur=${COMP_WORDS[COMP_CWORD]}
  offset=1
  chosen=${COMP_WORDS[1]}
  if [ -n "$chosen" ]; then
    if [ "$chosen" = "dotfiles" ]; then
      offset=2
    fi
  fi
  chosen=${COMP_WORDS[$offset]}
  subset=$((offset+1))
  if [ "$COMP_CWORD" -eq $offset ]; then
    opts="{{ $.Deploy }} {{ $.Diff }}"
  else
    if [ "$COMP_CWORD" -eq $subset ]; then
      case "$chosen" in
        "{{ $.Deploy }}")
          opts="{{ $.Args.DryRun }} {{ $.Args.Force }} {{ $.Args.Overwrite }}"
        ;;
        "{{ $.Diff }}")
          opts="{{ $.Args.Verbose }}"
        ;;
      esac
    else
      offset=$((subset+1))
      if [ "$COMP_CWORD" -eq $offset ]; then
        if [ "$chosen" = "{{ $.Deploy }}" ]; then
          chosen=${COMP_WORDS[$subset]}
          case "$chosen" in
            "{{ $.Args.Force }}" | "{{ $.Args.Overwrite }}")
                opts="{{ $.Args.DryRun }}"
              ;;
            "{{ $.Args.DryRun }}")
                opts="{{ $.Args.Force }} {{ $.Args.Overwrite }}"
              ;;
          esac
        fi
      fi
    fi
  fi
  if [ -n "$opts" ]; then
    # shellcheck disable=SC2207
    COMPREPLY=($(compgen -W "$opts" -- "$cur"))
  fi
}

complete -F _git_dotfiles -o bashdefault {{ $.Exe }}
