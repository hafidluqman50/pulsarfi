import { NavBar } from './NavBar';
import { PriceTicker } from './PriceTicker';
import { Footer } from './Footer';

interface LayoutProps {
  children: React.ReactNode;
}

export function Layout({ children }: LayoutProps) {
  return (
    <div className="min-h-screen flex flex-col">
      <NavBar />
      <PriceTicker />
      <main className="flex-1">
        {children}
      </main>
      <Footer />
    </div>
  );
}
