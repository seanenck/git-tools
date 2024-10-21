_blap() {
  local cur opts chosen
  cur=${COMP_WORDS[COMP_CWORD]}
  if [ "$COMP_CWORD" -eq 1 ]; then
    opts="{{ $.Deploy }} {{ $.Diff }}"
  else
    chosen=${COMP_WORDS[1]}
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

complete -F _blap -o bashdefault blap
