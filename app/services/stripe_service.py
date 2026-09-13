import stripe
from typing import Optional
from uuid import UUID
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select
from app.core.config import settings
from app.models import User, Organization, Subscription, SubscriptionTier, SubscriptionStatus
from app.core.security import create_access_token


stripe.api_key = settings.STRIPE_SECRET_KEY


class StripeService:
    def __init__(self, db: AsyncSession):
        self.db = db

    async def create_customer(self, user: User, organization: Organization) -> str:
        customer = stripe.Customer.create(
            email=user.email,
            name=organization.name,
            metadata={
                "user_id": str(user.id),
                "organization_id": str(organization.id),
            },
        )
        organization.stripe_customer_id = customer.id
        await self.db.commit()
        return customer.id

    async def create_checkout_session(
        self,
        organization: Organization,
        price_id: str,
        success_url: str,
        cancel_url: str,
    ) -> dict:
        if not organization.stripe_customer_id:
            # Get first user of org to create customer
            result = await self.db.execute(
                select(User).where(User.organization_id == organization.id).limit(1)
            )
            user = result.scalar_one_or_none()
            if user:
                await self.create_customer(user, organization)

        session = stripe.checkout.Session.create(
            customer=organization.stripe_customer_id,
            payment_method_types=["card"],
            line_items=[{"price": price_id, "quantity": 1}],
            mode="subscription",
            success_url=success_url,
            cancel_url=cancel_url,
            allow_promotion_codes=True,
            billing_address_collection="required",
            metadata={
                "organization_id": str(organization.id),
            },
        )
        return {"session_id": session.id, "url": session.url}

    async def create_portal_session(self, organization: Organization, return_url: str) -> str:
        if not organization.stripe_customer_id:
            raise ValueError("No Stripe customer for organization")
        
        session = stripe.billing_portal.Session.create(
            customer=organization.stripe_customer_id,
            return_url=return_url,
        )
        return session.url

    async def handle_subscription_created(self, stripe_sub: dict) -> None:
        org_id = stripe_sub.get("metadata", {}).get("organization_id")
        if not org_id:
            return

        result = await self.db.execute(
            select(Organization).where(Organization.id == UUID(org_id))
        )
        org = result.scalar_one_or_none()
        if not org:
            return

        sub = Subscription(
            organization_id=org.id,
            stripe_subscription_id=stripe_sub["id"],
            stripe_price_id=stripe_sub["items"]["data"][0]["price"]["id"],
            tier=self._map_price_to_tier(stripe_sub["items"]["data"][0]["price"]["id"]),
            status=SubscriptionStatus(stripe_sub["status"]),
            current_period_start=datetime.fromtimestamp(stripe_sub["current_period_start"]),
            current_period_end=datetime.fromtimestamp(stripe_sub["current_period_end"]),
            cancel_at_period_end=stripe_sub.get("cancel_at_period_end", False),
            trial_end=datetime.fromtimestamp(stripe_sub["trial_end"]) if stripe_sub.get("trial_end") else None,
        )
        self.db.add(sub)
        await self.db.commit()

    async def handle_subscription_updated(self, stripe_sub: dict) -> None:
        result = await self.db.execute(
            select(Subscription).where(Subscription.stripe_subscription_id == stripe_sub["id"])
        )
        sub = result.scalar_one_or_none()
        if not sub:
            return

        sub.status = SubscriptionStatus(stripe_sub["status"])
        sub.current_period_start = datetime.fromtimestamp(stripe_sub["current_period_start"])
        sub.current_period_end = datetime.fromtimestamp(stripe_sub["current_period_end"])
        sub.cancel_at_period_end = stripe_sub.get("cancel_at_period_end", False)
        sub.canceled_at = datetime.fromtimestamp(stripe_sub["canceled_at"]) if stripe_sub.get("canceled_at") else None
        sub.trial_end = datetime.fromtimestamp(stripe_sub["trial_end"]) if stripe_sub.get("trial_end") else None
        sub.stripe_price_id = stripe_sub["items"]["data"][0]["price"]["id"]
        sub.tier = self._map_price_to_tier(stripe_sub["items"]["data"][0]["price"]["id"])
        await self.db.commit()

    async def handle_subscription_deleted(self, stripe_sub: dict) -> None:
        result = await self.db.execute(
            select(Subscription).where(Subscription.stripe_subscription_id == stripe_sub["id"])
        )
        sub = result.scalar_one_or_none()
        if not sub:
            return

        sub.status = SubscriptionStatus.CANCELED
        sub.canceled_at = datetime.utcnow()
        await self.db.commit()

    async def handle_invoice_payment_failed(self, invoice: dict) -> None:
        sub_id = invoice.get("subscription")
        if not sub_id:
            return

        result = await self.db.execute(
            select(Subscription).where(Subscription.stripe_subscription_id == sub_id)
        )
        sub = result.scalar_one_or_none()
        if not sub:
            return

        sub.status = SubscriptionStatus.PAST_DUE
        await self.db.commit()

    def _map_price_to_tier(self, price_id: str) -> SubscriptionTier:
        if price_id == settings.STRIPE_PRICE_ID_MONTHLY:
            return SubscriptionTier.MONTHLY
        elif price_id == settings.STRIPE_PRICE_ID_YEARLY:
            return SubscriptionTier.YEARLY
        elif price_id == settings.STRIPE_PRICE_ID_ENTERPRISE:
            return SubscriptionTier.ENTERPRISE
        return SubscriptionTier.FREE


from datetime import datetime