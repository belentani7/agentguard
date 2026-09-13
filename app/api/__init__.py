from datetime import timedelta
from uuid import UUID
from fastapi import APIRouter, Depends, HTTPException, status, Request, Header
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select
from app.core.config import settings
from app.core.database import get_db
from app.core.security import verify_password, get_password_hash, create_access_token, decode_token
from app.models import User, Organization, Subscription, SubscriptionTier
from app.schemas import (
    UserCreate, UserResponse, UserUpdate, Token, TokenData,
    OrganizationCreate, OrganizationResponse,
    SubscriptionResponse, CheckoutSessionRequest, CheckoutSessionResponse,
    PortalSessionRequest, PortalSessionResponse, WebhookEvent, HealthResponse
)
from app.services.stripe_service import StripeService
import stripe


router = APIRouter()


async def get_current_user(
    token: str = Depends(lambda x: x.headers.get("Authorization", "").replace("Bearer ", "")),
    db: AsyncSession = Depends(get_db)
) -> User:
    if not token:
        raise HTTPException(status_code=401, detail="Not authenticated")
    
    payload = decode_token(token)
    if not payload or "sub" not in payload:
        raise HTTPException(status_code=401, detail="Invalid token")
    
    user_id = UUID(payload["sub"])
    result = await db.execute(select(User).where(User.id == user_id))
    user = result.scalar_one_or_none()
    
    if not user or not user.is_active:
        raise HTTPException(status_code=401, detail="User not found or inactive")
    
    return user


async def get_current_org(
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
) -> Organization:
    if not current_user.organization_id:
        raise HTTPException(status_code=400, detail="User not in organization")
    
    result = await db.execute(
        select(Organization).where(Organization.id == current_user.organization_id)
    )
    org = result.scalar_one_or_none()
    
    if not org:
        raise HTTPException(status_code=404, detail="Organization not found")
    
    return org


@router.post("/auth/register", response_model=UserResponse, status_code=201)
async def register(user_data: UserCreate, db: AsyncSession = Depends(get_db)):
    result = await db.execute(select(User).where(User.email == user_data.email))
    if result.scalar_one_or_none():
        raise HTTPException(status_code=400, detail="Email already registered")
    
    hashed_pw = get_password_hash(user_data.password)
    user = User(
        email=user_data.email,
        full_name=user_data.full_name,
        hashed_password=hashed_pw,
    )
    db.add(user)
    await db.commit()
    await db.refresh(user)
    return user


@router.post("/auth/login", response_model=Token)
async def login(email: str, password: str, db: AsyncSession = Depends(get_db)):
    result = await db.execute(select(User).where(User.email == email))
    user = result.scalar_one_or_none()
    
    if not user or not verify_password(password, user.hashed_password):
        raise HTTPException(status_code=401, detail="Invalid credentials")
    
    if not user.is_active:
        raise HTTPException(status_code=401, detail="Account deactivated")
    
    access_token = create_access_token(data={"sub": str(user.id)})
    return Token(access_token=access_token)


@router.get("/auth/me", response_model=UserResponse)
async def get_me(current_user: User = Depends(get_current_user)):
    return current_user


@router.patch("/auth/me", response_model=UserResponse)
async def update_me(
    user_update: UserUpdate,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    if user_update.full_name is not None:
        current_user.full_name = user_update.full_name
    if user_update.is_active is not None:
        current_user.is_active = user_update.is_active
    
    await db.commit()
    await db.refresh(current_user)
    return current_user


@router.post("/organizations", response_model=OrganizationResponse, status_code=201)
async def create_organization(
    org_data: OrganizationCreate,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    if current_user.organization_id:
        raise HTTPException(status_code=400, detail="User already in organization")
    
    slug = org_data.name.lower().replace(" ", "-")[:100]
    result = await db.execute(select(Organization).where(Organization.slug == slug))
    if result.scalar_one_or_none():
        slug = f"{slug}-{current_user.id.hex[:8]}"
    
    org = Organization(name=org_data.name, slug=slug)
    db.add(org)
    await db.flush()
    
    current_user.organization_id = org.id
    await db.commit()
    await db.refresh(org)
    return org


@router.get("/organizations/me", response_model=OrganizationResponse)
async def get_my_organization(
    org: Organization = Depends(get_current_org)
):
    return org


@router.post("/billing/checkout", response_model=CheckoutSessionResponse)
async def create_checkout(
    request: CheckoutSessionRequest,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    org_id = request.organization_id or current_user.organization_id
    if not org_id:
        raise HTTPException(status_code=400, detail="No organization")
    
    result = await db.execute(select(Organization).where(Organization.id == org_id))
    org = result.scalar_one_or_none()
    if not org:
        raise HTTPException(status_code=404, detail="Organization not found")
    
    stripe_svc = StripeService(db)
    session_data = await stripe_svc.create_checkout_session(
        org, request.price_id, request.success_url, request.cancel_url
    )
    return CheckoutSessionResponse(**session_data)


@router.post("/billing/portal", response_model=PortalSessionResponse)
async def create_portal(
    request: PortalSessionRequest,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    org = await get_current_org(current_user, db)
    stripe_svc = StripeService(db)
    url = await stripe_svc.create_portal_session(org, request.return_url)
    return PortalSessionResponse(url=url)


@router.get("/billing/subscription", response_model=SubscriptionResponse)
async def get_subscription(
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    org = await get_current_org(current_user, db)
    result = await db.execute(
        select(Subscription).where(Subscription.organization_id == org.id)
    )
    sub = result.scalar_one_or_none()
    
    if not sub:
        return SubscriptionResponse(
            id=UUID("00000000-0000-0000-0000-000000000000"),
            tier=SubscriptionTier.FREE.value,
            status=SubscriptionStatus.INCOMPLETE.value,
            cancel_at_period_end=False,
            created_at=datetime.utcnow(),
        )
    
    return sub


@router.post("/webhooks/stripe")
async def stripe_webhook(
    request: Request,
    stripe_signature: str = Header(None),
    db: AsyncSession = Depends(get_db)
):
    payload = await request.body()
    
    try:
        event = stripe.Webhook.construct_event(
            payload, stripe_signature, settings.STRIPE_WEBHOOK_SECRET
        )
    except ValueError:
        raise HTTPException(status_code=400, detail="Invalid payload")
    except stripe.error.SignatureVerificationError:
        raise HTTPException(status_code=400, detail="Invalid signature")
    
    stripe_svc = StripeService(db)
    
    if event["type"] == "checkout.session.completed":
        session = event["data"]["object"]
        if session.get("subscription"):
            stripe_sub = stripe.Subscription.retrieve(session["subscription"])
            await stripe_svc.handle_subscription_created(stripe_sub)
    
    elif event["type"] == "customer.subscription.created":
        await stripe_svc.handle_subscription_created(event["data"]["object"])
    
    elif event["type"] == "customer.subscription.updated":
        await stripe_svc.handle_subscription_updated(event["data"]["object"])
    
    elif event["type"] == "customer.subscription.deleted":
        await stripe_svc.handle_subscription_deleted(event["data"]["object"])
    
    elif event["type"] == "invoice.payment_failed":
        await stripe_svc.handle_invoice_payment_failed(event["data"]["object"])
    
    return {"received": True}


@router.get("/health", response_model=HealthResponse)
async def health():
    return HealthResponse(
        status="healthy",
        version="1.0.0",
        environment=settings.ENVIRONMENT
    )


from datetime import datetime