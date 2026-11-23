# PHP repo and toolchain

Digest of `specs/languages/php/RULES.md` REPO and TOOL. Project files win over this (log overrides).

## Layout (`PHP-REPO-01`, `-03`)
One `composer.json` per deployable at the root. `src/` is PSR-4 `App\`; `tests/` is PSR-4 `autoload-dev`. Public entry only `public/index.php`. Start from the official skeleton (`composer create-project laravel/laravel`, `symfony new`), or `composer init` for a framework-free service.
Day-one files: `composer.lock`, `phpunit.xml.dist`, `phpstan.neon.dist`, `.php-cs-fixer.dist.php` (or `pint.json`), `deptrac.php`, CI, `.editorconfig`, `.env.example`, `.php-version`.

## Composer
- Commit `composer.lock` for apps and libraries. CI and the gate run `composer install`, never `update`; bump deliberately with `composer update vendor/pkg` (`PHP-REPO-04`).
- `config.platform.php` equals the production runtime; pair with `composer check-platform-reqs` (`PHP-REPO-05`).
- `require` for runtime, `require-dev` for tooling, `sort-packages: true`, caret constraints, no `dev-main` in apps (`PHP-REPO-06`).
- `config.allow-plugins` is an object of trusted plugins, never `true` (`PHP-REPO-07`).
- `composer validate --strict` is a required gate check; it also flags a stale lock.
- Monorepo: one root `composer.json` with `path` repositories; never share `vendor/` (`PHP-REPO-02`).
- Libraries: SemVer tags `vX.Y.Z`, no `version` key, `CHANGELOG.md` (`PHP-REPO-09`). Conventional commits as in loops (`PHP-REPO-14`).

## Ignore files (`PHP-REPO-11`)
Ignore `/vendor/`, `.env`, `/.phpunit.cache/`, `.phpunit.result.cache`, `.php-cs-fixer.cache`, `/var/`, `/bootstrap/cache/*.php`, `/coverage/`. `.gitattributes`: `*.php text eol=lf`, `export-ignore` tests.

## Toolchain (`PHP-TOOL-*`)
- Versions: PHP 8.4/8.5; 8.2 security support ends 2026-12-31 (`PHP-TOOL-01`). PHPUnit 12 needs 8.3+, 13 needs 8.4+.
- Format: PER-CS via PHP-CS-Fixer `@PER-CS` or Pint; one formatter per repo; do not add PHPCS on the same rules (`-02`, `-03`).
- PHPStan level 8+ for new code, 6 minimum for legacy with a baseline; pin a number, not `max` (`-04`). Shrink the baseline only (`-05`). Psalm is an alternative, not an addition (`-06`).
- Rector: `process --dry-run` in CI, never unattended on a red tree (`-07`).
- `composer audit --locked` (exit 1 on advisories); `config.policy.advisories.block` stays on (`-08`). Gate: `--vuln`.
- SBOM: `cyclonedx/cyclonedx-php-composer`; licenses: `composer licenses` (`-09`).

## CI order (`PHP-REPO-12`)
validate -> install -> format check -> PHPStan -> `deptrac analyse` -> PHPUnit with coverage -> `composer audit --locked`. Matrix over supported PHP minors; `--prefer-lowest` for libraries. Cache `COMPOSER_CACHE_DIR` keyed on the lock hash, not `vendor/` (`-13`).

## Wave 0 files owned by `php-engineer`
Manifests and lock, `phpunit.xml.dist` (source include, strictness), PHPStan/CS/deptrac configs, coverage driver note, CI, ignore files, version pins.
