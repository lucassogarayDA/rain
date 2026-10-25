#!/usr/bin/env bash
# Calcula la siguiente versión semver a partir de los commits desde el último
# tag, usando Conventional Commits:
#
#   BREAKING CHANGE / feat!:  -> major
#   feat:                     -> minor
#   fix:/perf:/refactor:/...  -> patch
#   cualquier otro commit     -> patch (si hay commits desde el último tag)
#
# Uso:
#   scripts/next-version.sh            # imprime la siguiente versión (vX.Y.Z)
#   scripts/next-version.sh --bump     # indica si hay que subir (0) o no (1)
#
# Salida por stdout: la versión (p. ej. v0.2.0). Diagnóstico por stderr.
set -euo pipefail

last_tag="$(git describe --tags --abbrev=0 2>/dev/null || true)"

if [[ -z "${last_tag}" ]]; then
  # Primer release: empezar en v0.1.0.
  echo "v0.1.0"
  exit 0
fi

# Base: el tag sin la "v".
base="${last_tag#v}"
IFS='.' read -r major minor patch <<<"${base}"
major="${major:-0}"
minor="${minor:-0}"
patch="${patch:-0}"

# Commits desde el último tag (ignora merges).
log="$(git log "${last_tag}..HEAD" --no-merges --pretty=format:'%s%n%b%n---END---' 2>/dev/null || true)"

bump="none"
if grep -qE '^[a-z]+(\([^)]*\))?!:|BREAKING[ -]CHANGE' <<<"${log}"; then
  bump="major"
elif grep -qE '^feat(\([^)]*\))?:' <<<"${log}"; then
  bump="minor"
elif grep -qE '^(fix|perf|refactor|revert|build|deps)(\([^)]*\))?:' <<<"${log}"; then
  bump="patch"
elif [[ -n "${log}" ]]; then
  bump="patch"
fi

case "${bump}" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
  none) : ;;
esac

next="v${major}.${minor}.${patch}"

# Modo --bump: exit 0 si hay nueva versión, 1 si no cambia.
if [[ "${1:-}" == "--bump" ]]; then
  if [[ "${next}" == "${last_tag}" ]]; then
    exit 1
  fi
  exit 0
fi

echo "${next}"
