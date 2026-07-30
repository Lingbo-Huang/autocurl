# Autocurl website

The website is a dependency-free static site. Preview it from the repository
root:

```bash
python3 -m http.server 4173 --directory site
```

Then open <http://127.0.0.1:4173/>.

## Hosting

- GitHub Pages deploys `site/` through `.github/workflows/pages.yml`.
- Vercel can import the repository directly. `vercel.json` sets `site/` as the
  static output directory.
- All page links and assets are relative, so the same files work from the
  GitHub Pages `/autocurl/` subpath and a Vercel root domain.

The site has no analytics, cookies, accounts, or build-time dependencies.
