## 9. Using shadcn Components

### 9.1 What is shadcn?

- **Not a traditional component library** — it's a copy/paste collection. Components are added to your source tree, not installed as a dependency.
- **Built on Radix UI primitives** — handles accessibility, keyboard navigation, focus management, and ARIA attributes out of the box.
- **Components live in your repo** — under `src/components/ui/`. Fully editable since the code is yours.
- **Requires Tailwind CSS** — styling uses Tailwind utility classes plus CSS variables for theming.
- **Build tool agnostic** — works with Vite, Next.js, Astro, Remix, or any framework that supports React + Tailwind.

### 9.2 Setup

```bash
npx shadcn@latest init
```

This command:

1. Creates a `components.json` file (configuration for shadcn)
2. Adds CSS variable tokens to your global CSS (`--background`, `--foreground`, `--primary`, `--radius`, etc.)
3. Installs `tailwind-merge`, `clsx`, and `class-variance-authority`
4. Sets up a `cn()` utility in `src/lib/utils.ts`

**Path alias (`@/`):** Ensure your `vite.config.ts` (or `tsconfig.json`) has the `@` alias pointing to `src/`:

```typescript
// vite.config.ts
resolve: {
  alias: {
    '@': '/src',
  },
}
```

This enables imports like:

```typescript
import { Button } from '@/components/ui/button'
```

### 9.3 Adding Components

```bash
# Single component
npx shadcn@latest add button

# Multiple at once
npx shadcn@latest add dialog dropdown-menu form input select toast
```

Each command creates a single file (or a small directory) inside `src/components/ui/`. Components are self-contained and have zero internal dependencies between shadcn files.

### 9.4 Customization — Two Approaches

**Approach 1: CSS Variables (theming)**

Edit the `:root` block in your global CSS to change theme-wide tokens:

```css
:root {
  --primary: 221.2 83.2% 53.3%;
  --primary-foreground: 210 40% 98%;
  --radius: 0.5rem;
}

.dark {
  --primary: 220 70% 55%;
}
```

Values use HSL color format. Change these once and every shadcn component updates.

**Approach 2: Tailwind classes (one-off overrides)**

Every shadcn component accepts a `className` prop:

```tsx
<Button className="bg-red-500 hover:bg-red-600 text-white">
  Delete
</Button>
```

**Approach 3: Edit the source directly**

Since the code is in your own repo, you can change any component's internals. This is useful for adding a new variant that doesn't fit shadcn's existing patterns. However, see 9.8 for upgrade caveats.

### 9.5 Common Components Reference

| Component | Best For | Radix Primitive |
|---|---|---|
| `Button` | Primary/secondary actions, form submission | — (custom) |
| `Input` / `Textarea` | Text entry fields | — (custom) |
| `Label` | Accessible form labels | `@radix-ui/react-label` |
| `Form` | Forms with validation + error display | wraps `react-hook-form` |
| `Card` | Grouped content sections (header, content, footer) | — (custom) |
| `Dialog` | Modals, confirmations, quick actions | `@radix-ui/react-dialog` |
| `AlertDialog` | Destructive confirmations ("Are you sure?") | `@radix-ui/react-alert-dialog` |
| `DropdownMenu` | Context menus, user avatar menus | `@radix-ui/react-dropdown-menu` |
| `Select` | Dropdown selection from a list | `@radix-ui/react-select` |
| `Badge` | Status indicators, tags, counters | — (custom) |
| `Toast` | Transient notifications (success, error, info) | `@radix-ui/react-toast` |
| `Separator` | Horizontal/vertical dividers | `@radix-ui/react-separator` |
| `Tabs` | Tabbed content sections | `@radix-ui/react-tabs` |
| `Table` | Data tables (not data grids — use TanStack Table for that) | — (custom) |
| `Sheet` | Slide-in panels (mobile nav, settings drawers) | `@radix-ui/react-dialog` |
| `Skeleton` | Loading placeholders | — (custom) |
| `Tooltip` | Hover/tap tooltips | `@radix-ui/react-tooltip` |

### 9.6 Comparison: shadcn vs. Traditional UI Libraries

| Aspect | shadcn | MUI / Chakra / Ant Design |
|---|---|---|
| Install | Copy into source | `npm install package` |
| Bundle size | Only what you add | Full library |
| Customization | Anywhere (CSS vars, Tailwind, source edit) | Theming API only |
| Upgrades | Re-copy the file | `npm update` |
| Control | Full — you own the code | Limited to library API |
| Accessibility | Radix primitives reinforce it | Built-in but fixed |
| Learning curve | Need Tailwind knowledge | Learn their prop/theme API |

### 9.7 Composition Patterns

**Dialog + Button + Form**

```tsx
<Dialog>
  <DialogTrigger asChild>
    <Button>New Item</Button>
  </DialogTrigger>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>Create Item</DialogTitle>
      <DialogDescription>Add a new item to your list.</DialogDescription>
    </DialogHeader>
    <form>
      <Input placeholder="Name" />
      <Button type="submit">Save</Button>
    </form>
  </DialogContent>
</Dialog>
```

**DropdownMenu + Button asChild**

```tsx
<DropdownMenu>
  <DropdownMenuTrigger asChild>
    <Button variant="ghost">Options</Button>
  </DropdownMenuTrigger>
  <DropdownMenuContent>
    <DropdownMenuItem>Edit</DropdownMenuItem>
    <DropdownMenuItem>Duplicate</DropdownMenuItem>
    <DropdownMenuSeparator />
    <DropdownMenuItem className="text-red-500">Delete</DropdownMenuItem>
  </DropdownMenuContent>
</DropdownMenu>
```

**Form with validation (react-hook-form + zod)**

```tsx
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'

const schema = z.object({
  email: z.string().email(),
  name: z.string().min(2),
})

type FormData = z.infer<typeof schema>

function MyForm() {
  const form = useForm<FormData>({
    resolver: zodResolver(schema),
  })

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit((data) => console.log(data))}>
        <FormField
          control={form.control}
          name="email"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Email</FormLabel>
              <FormControl>
                <Input placeholder="you@example.com" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button type="submit">Submit</Button>
      </form>
    </Form>
  )
}
```

**Card + Badge (feature card)**

```tsx
<Card>
  <CardHeader>
    <div className="flex items-center justify-between">
      <CardTitle>Pro Plan</CardTitle>
      <Badge>Popular</Badge>
    </div>
    <CardDescription>For growing teams</CardDescription>
  </CardHeader>
  <CardContent>
    <p>$29/month</p>
  </CardContent>
  <CardFooter>
    <Button className="w-full">Upgrade</Button>
  </CardFooter>
</Card>
```

**Toast after async action**

```tsx
import { useToast } from '@/hooks/use-toast'

function SaveButton() {
  const { toast } = useToast()

  async function handleSave() {
    try {
      await saveData()
      toast({ title: 'Saved', description: 'Your changes were saved.' })
    } catch {
      toast({ variant: 'destructive', title: 'Error', description: 'Something went wrong.' })
    }
  }

  return <Button onClick={handleSave}>Save</Button>
}
```

### 9.8 Best Practices

1. **Do NOT modify files in `src/components/ui/` directly** — extend via composition in your own components. This makes re-adding on shadcn upgrades safe. If you need a custom variant, create a wrapper component in `src/components/your-component.tsx`.

2. **Use `asChild` for polymorphic composition** — Radix's `asChild` lets shadcn components render as any element:

   ```tsx
   <Button asChild>
     <Link href="/dashboard">Dashboard</Link>
   </Button>
   ```

3. **One file per component** — matches the shadcn convention and keeps imports predictable.

4. **Import from `@/components/ui/`** — clean aliased imports regardless of how deep the file lives.

5. **Forms: use the canonical stack** — `react-hook-form` + `zod` + `@hookform/resolvers` is what shadcn's `<Form>` component wraps. Don't roll your own validation unless you must.

6. **Prefer CSS variables for theme tokens** — set `--primary`, `--muted`, `--radius`, etc. in `globals.css`. Use Tailwind utility classes for one-off overrides only.

7. **Add a `use-toast` hook** — when you first add `toast`, shadcn creates `@/hooks/use-toast`. Keep it there — many components reference it.

### 9.9 Caveats & Gotchas

- **Re-adding a component overwrites your edits** — if you modified a shadcn source file directly, running `npx shadcn add button` again will replace your changes. Keep custom variants in separate wrapper files.
- **Not every Radix primitive is wrapped** — check the [shadcn catalog](https://ui.shadcn.com/docs/components) before hand-rolling. If it's not there, consider using the Radix primitive directly.
- **CSS variables vs. Tailwind** — theme tokens in CSS variables are global; Tailwind classes are local. Don't mix both for the same property on the same element — it makes debugging unpredictable.
- **`cn()` utility** — shadcn generates a `cn()` helper in `src/lib/utils.ts` that merges `clsx` + `tailwind-merge`. Use it anywhere you need to conditionally merge classes:

  ```tsx
  cn('text-base', isLarge && 'text-lg', className)
  ```

- **Lucide React icons** — shadcn uses `lucide-react` for icons. Install it explicitly: `npm install lucide-react`.
- **Dark mode** — requires adding a `.dark` class toggling mechanism. shadcn components read `@media (prefers-color-scheme: dark)` by default but can be driven by a class toggle. See `next-themes` or write a simple React context.