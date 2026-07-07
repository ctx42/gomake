#!/usr/bin/env bash

_gomake() {
  # _get_comp_words_by_ref and __ltrim_colon_completions come from the
  # bash-completion package; without it there is nothing to complete.
  declare -F _get_comp_words_by_ref >/dev/null || return

  local cur prev words
  _get_comp_words_by_ref -n : cur prev

  # https://www.gnu.org/savannah-checkouts/gnu/bash/manual/bash.html#index-COMP_005fCWORD

  # An index into ${COMP_WORDS} of the word containing the current cursor
  # position.
  export COMP_CWORD

  # The current command line.
  export COMP_LINE

  # The index of the current cursor position relative to the beginning of
  # the current command. If the current cursor position is at the end of
  # the current command, the value of this variable is equal to ${#COMP_LINE}.
  export COMP_POINT

  # Set to an integer value corresponding to the type of completion attempted
  # that caused a completion function to be called: TAB, for normal completion,
  # ‘?’, for listing completions after successive tabs, ‘!’, for listing
  # alternatives on partial word completion, ‘@’, to list completions if the
  # word is not unmodified, or ‘%’, for menu completion.
  export COMP_TYPE

  # The key (or final key of a key sequence) used to invoke the current
  # completion function.
  export COMP_KEY

  # The set of characters that the Readline library treats as word separators
  # when performing word completion. If COMP_WORDBREAKS is unset, it loses its
  # special properties, even if it is subsequently reset.
  export COMP_WORDBREAKS

  # An array variable consisting of the individual words in the current
  # command line. The line is split into words as Readline would split it,
  # using COMP_WORDBREAKS as described above.
  export COMP_WORDS

  # An array variable from which Bash reads the possible completions generated
  # by a shell function invoked by the programmable completion facility
  # (see Programmable Completion). Each array element contains one possible
  # completion.
  COMPREPLY=()

  # Get word completions from gomake itself. $1 is the command name bash
  # passes to the completion function. gomake's completion path expects
  # exactly three args (name, current, previous), so forwarding it fills the
  # name slot; it also stops Go's flag parser at the first non-flag token,
  # keeping a leading-dash "$cur" from being parsed as a flag.
  words=$(gomake "$1" "$cur" "$prev")

  # shellcheck disable=SC2207
  COMPREPLY=($(COMP_WORDBREAKS=${COMP_WORDBREAKS//:/} compgen -W "${words}" -- "${cur}"))

  # Debugging.
  #{
  #  echo "cur: $cur"
  #  echo "prev: $prev"
  #  echo "words: $words"
  #  echo "cword: $cword"
  #  echo "COMP_CWORD: $COMP_CWORD"
  #  echo "COMP_LINE: $COMP_LINE"
  #  echo "COMP_POINT: $COMP_POINT"
  #  echo "COMP_TYPE: $COMP_TYPE"
  #  echo "COMP_KEY: $COMP_KEY"
  #  echo "COMP_WORDBREAKS: $COMP_WORDBREAKS"
  #  echo "COMP_WORDS: $COMP_WORDS"
  #  echo "1: $1"
  #  echo "2: $2"
  #  echo "3: $3"
  #  echo "words: $words"
  #  echo "---"
  #} >>tmp/_gomake_comp_fn.txt

  __ltrim_colon_completions "$cur"
}
complete -F _gomake gomake
