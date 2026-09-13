from datetime import datetime
from typing import Optional
from pydantic import BaseModel, EmailStr, Field
import uuid


class UserBase(BaseModel):
    email: EmailStr
    full_name: Optional[str] = None


class UserCreate(UserBase):
    password: str = Field(..., min_length=8)


class UserUpdate(BaseModel):
    full_name: Optional[str] = None
    is_active: Optional[bool] = None


class UserResponse(UserBase):
    id: uuid.UUID
    is_active: bool
    is_superuser: bool
    created_at: datetime
    organization_id: Optional[uuid.UUID] = None

    class Config:
        from_attributes = True


class Token(BaseModel):
    access_token: str
    token_type: str = "bearer"


class TokenData(BaseModel):
    email: Optional[str] = None
    user_id: Optional[uuid.UUID] = None


class OrganizationBase(BaseModel):
    name: str = Field(..., min_length=1, max_length=255)


class OrganizationCreate(OrganizationBase):
    pass


class OrganizationResponse(OrganizationBase):
    id: uuid.UUID
    slug: str
    stripe_customer_id: Optional[str] = None
    created_at: datetime

    class Config:
        from_attributes = True


class SubscriptionBase(BaseModel):
    tier: str
    status: str


class SubscriptionResponse(SubscriptionBase):
    id: uuid.UUID
    stripe_subscription_id: Optional[str] = None
    stripe_price_id: Optional[str] = None
    current_period_start: Optional[datetime] = None
    current_period_end: Optional[datetime] = None
    cancel_at_period_end: bool
    canceled_at: Optional[datetime] = None
    trial_end: Optional[datetime] = None
    created_at: datetime

    class Config:
        from_attributes = True


class CheckoutSessionRequest(BaseModel):
    price_id: str
    success_url: str
    cancel_url: str
    organization_id: Optional[uuid.UUID] = None


class CheckoutSessionResponse(BaseModel):
    session_id: str
    url: str


class PortalSessionRequest(BaseModel):
    return_url: str


class PortalSessionResponse(BaseModel):
    url: str


class WebhookEvent(BaseModel):
    id: str
    type: str
    data: dict
    created: int


class HealthResponse(BaseModel):
    status: str
    version: str
    environment: str