import { HeroSection } from '../components/HeroSection'
import { InstallSection } from '../components/InstallSection'
import { PreviewImageLink } from '../components/PreviewImageLink'
import { SiteFooter } from '../components/SiteFooter'
import { StatusSection } from '../components/StatusSection'
import { WhySection } from '../components/WhySection'

export function HomePage() {
  return (
    <>
      <main className="w-full max-w-3xl px-5 pb-24 md:ml-16 md:px-0">
        <HeroSection />
        <PreviewImageLink
          href="/demo.png"
          src="/demo.png"
          alt="Termtap demo preview"
          caption="Demo screenshot preview."
        />
        <WhySection />
        <StatusSection />
        <InstallSection />
      </main>
      <SiteFooter />
    </>
  )
}
