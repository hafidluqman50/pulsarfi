import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';

const cards = [
  {
    title: 'Why PulsarFi exists',
    href: '/docs/problem',
    text: 'The market access, settlement, liquidity, and compliance constraints that shape the product.',
  },
  {
    title: 'Architecture',
    href: '/docs/architecture',
    text: 'The trust boundaries between smart contracts, frontend, backend, custodians, storage, and market data.',
  },
  {
    title: 'Protocol design',
    href: '/docs/protocol-design',
    text: 'The on-chain roles, invariants, proposal lifecycle, KYC gateway, and pool mechanics.',
  },
  {
    title: 'Business flow',
    href: '/docs/business-flow',
    text: 'How custody, minting, liquidity provisioning, trading, redemption, and reserve records fit together.',
  },
  {
    title: 'App flow',
    href: '/docs/app-flow',
    text: 'Detailed user, trader, portfolio, custodian, KYC, and redemption journeys.',
  },
  {
    title: 'Contracts and faucet',
    href: '/docs/contracts-faucet',
    text: 'Live Arbitrum Sepolia addresses plus a wallet button for claiming testnet IDRX.',
  },
];

export default function Home(): JSX.Element {
  return (
    <Layout
      title="PulsarFi Docs"
      description="PulsarFi documentation, tokenized Indonesian equities on Arbitrum Sepolia"
    >
      <header className="hero">
        <div className="container">
          <Heading as="h1" className="hero__title">
            PulsarFi Documentation
          </Heading>
          <p className="hero__subtitle">
            A product and engineering guide for PulsarFi: why the system
            exists, what it guarantees, how the architecture is composed, how
            business operations flow, and how users interact with the protocol.
          </p>
          <div className="faucet-actions">
            <Link className="button button--primary button--lg" to="/docs/overview">
              Start reading
            </Link>
            <Link className="button button--secondary button--lg" to="/docs/contracts-faucet">
              Mint testnet IDRX
            </Link>
          </div>
        </div>
      </header>
      <main className="container">
        <div className="docs-card-grid">
          {cards.map((card) => (
            <Link key={card.href} className={clsx('docs-card')} to={card.href}>
              <strong>{card.title}</strong>
              <span>{card.text}</span>
            </Link>
          ))}
        </div>
      </main>
    </Layout>
  );
}
