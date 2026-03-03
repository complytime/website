# ComplyTime Website

The official website for [ComplyTime](https://github.com/complytime) - Cloud Native Compliance, Reimagined.

Built with [Hugo](https://gohugo.io/) and the [Doks](https://getdoks.org/) theme.

## 🚀 Quick Start

### Prerequisites

- [Node.js](https://nodejs.org/) v20.11 or later
- [npm](https://www.npmjs.com/) (included with Node.js)

### Development

```bash
# Install dependencies
npm install

# Start development server
npm run dev
```

The site will be available at `http://localhost:1313/`.

### Build

```bash
# Build for production
npm run build
```

The output will be in the `public/` directory.

## 📁 Project Structure

```
website/
├── assets/                # SCSS, JavaScript, images
│   ├── js/
│   │   └── custom.js
│   └── scss/
│       └── common/
│           ├── _custom.scss
│           └── _variables-custom.scss
├── config/                # Hugo configuration
│   ├── _default/
│   │   ├── hugo.toml
│   │   ├── languages.toml
│   │   ├── params.toml
│   │   └── menus/
│   │       └── menus.en.toml
│   ├── production/        # Production overrides
│   └── next/              # Alternative env overrides
├── content/               # Markdown content
│   ├── _index.md          # Homepage
│   ├── docs/              # Documentation
│   │   ├── getting-started/
│   │   └── projects/      # Project pages (complyctl, complyscribe, etc.)
│   └── privacy.md
├── layouts/               # Custom layouts
│   ├── home.html          # Homepage layout
│   └── docs/
│       └── list.html      # Docs section listing layout
├── static/                # Static assets (favicons, icons)
├── .github/
│   └── workflows/
│       └── deploy-gh-pages.yml  # CI/CD deployment
└── package.json
```

## 📝 Content

### Navigation

| Menu Item     | URL                    | Description                |
|---------------|------------------------|----------------------------|
| Getting Started | `/docs/getting-started/` | Documentation landing page |
| Projects      | `/docs/projects/`      | ComplyTime project pages   |
| Privacy Policy | `/privacy/`            | Privacy policy             |

### Adding Documentation

Create a new Markdown file in the appropriate directory under `content/docs/`:

```markdown
---
title: "Page Title"
description: "Page description"
lead: "Brief intro text"
date: 2024-01-01T00:00:00+00:00
draft: false
weight: 100
toc: true
---

Your content here...
```

## 🎨 Customization

### Styling

Custom styles are in `assets/scss/common/`:
- `_variables-custom.scss` - Variables and theme colors
- `_custom.scss` - Additional custom styles

### Configuration

Site configuration is in `config/_default/`:
- `hugo.toml` - Hugo settings
- `languages.toml` - Language and footer settings
- `params.toml` - Theme parameters
- `menus/menus.en.toml` - Navigation menus

## 🚢 Deployment

The site is deployed to GitHub Pages via the `.github/workflows/deploy-gh-pages.yml` workflow. On push to the configured branch, GitHub Actions builds the site with Hugo and deploys the `public/` directory.

## 🤝 Contributing

Contributions are welcome! Please see our [Contributing Guide](https://github.com/complytime/community).

## 📄 License

This website is licensed under [Apache 2.0](LICENSE).

## 🔗 Links

- [ComplyTime GitHub](https://github.com/complytime)
- [Doks Theme](https://getdoks.org/)
- [Hugo](https://gohugo.io/)