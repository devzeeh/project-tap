package smtp

import (
	"fmt"
	"time"
)

func SignupOTPCode() string {
	return `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Verify Your Email</title>
	<style>
		body { font-family: 'Inter', Arial, sans-serif; background-color: #f4f4f5; margin: 0; padding: 0; }
		.container { max-width: 600px; margin: 40px auto; background-color: #ffffff; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); overflow: hidden; }
		.header { background-color: #0f172a; color: #ffffff; padding: 20px; text-align: center; }
		.header h1 { margin: 0; font-size: 24px; font-weight: 600; }
		.content { padding: 30px; color: #334155; line-height: 1.6; }
		.content p { margin: 0 0 15px; }
		.otp-container { text-align: center; margin: 30px 0; }
		.otp-code { display: inline-block; font-size: 32px; font-weight: 700; letter-spacing: 5px; color: #2563eb; background-color: #eff6ff; padding: 15px 25px; border-radius: 6px; border: 1px dashed #bfdbfe; }
		.footer { background-color: #f8fafc; padding: 15px; text-align: center; font-size: 12px; color: #64748b; border-top: 1px solid #e2e8f0; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>Unicard</h1>
		</div>
		<div class="content">
			<p>Hello %s,</p>
			<p>Thank you for signing up for Unicard! To verify your email address and continue with the registration, please use the following One-Time Password (OTP). This OTP is valid for <strong>5 minutes</strong>.</p>
			<div class="otp-container">
				<div class="otp-code">%s</div>
			</div>
			<p>If you did not initiate this signup process, please ignore this email.</p>
			<p>Thank you,<br>The Unicard Team</p>
		</div>
		<div class="footer">
			&copy; ` + Year() + ` Unicard. All rights reserved.
		</div>
	</div>
</body>
</html>`
}

func WelcomeEmail() string {
	return `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Welcome to Unicard!</title>
	<style>
		body { font-family: 'Inter', Arial, sans-serif; background-color: #f4f4f5; margin: 0; padding: 0; }
		.container { max-width: 600px; margin: 40px auto; background-color: #ffffff; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); overflow: hidden; }
		.header { background-color: #0f172a; color: #ffffff; padding: 20px; text-align: center; }
		.header h1 { margin: 0; font-size: 24px; font-weight: 600; }
		.content { padding: 30px; color: #334155; line-height: 1.6; }
		.content p { margin: 0 0 15px; }
		.btn-container { text-align: center; margin: 30px 0; }
		.btn { display: inline-block; background-color: #2563eb; color: #ffffff; text-decoration: none; padding: 12px 24px; border-radius: 6px; font-weight: 600; }
		.footer { background-color: #f8fafc; padding: 15px; text-align: center; font-size: 12px; color: #64748b; border-top: 1px solid #e2e8f0; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>Unicard</h1>
		</div>
		<div class="content">
			<p>Hello %s,</p>
			<p>Welcome to Unicard! Your account has been successfully created and your card is now active.</p>
			<p>We are thrilled to have you onboard. You can now login to your dashboard to view your transactions, manage your account, and explore all the features Unicard has to offer.</p>
			<div class="btn-container">
				<a href="%s/login" class="btn">Login to Your Account</a>
			</div>
			<p>If you have any questions or need assistance, our support team is always here to help.</p>
			<p>Thank you,<br>The Unicard Team</p>
		</div>
		<div class="footer">
			&copy; ` + Year() + ` Unicard. All rights reserved.
		</div>
	</div>
</body>
</html>`
}

func OTPCode() string {
	return `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Password Reset OTP</title>
	<style>
		body { font-family: 'Inter', Arial, sans-serif; background-color: #f4f4f5; margin: 0; padding: 0; }
		.container { max-width: 600px; margin: 40px auto; background-color: #ffffff; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); overflow: hidden; }
		.header { background-color: #0f172a; color: #ffffff; padding: 20px; text-align: center; }
		.header h1 { margin: 0; font-size: 24px; font-weight: 600; }
		.content { padding: 30px; color: #334155; line-height: 1.6; }
		.content p { margin: 0 0 15px; }
		.otp-container { text-align: center; margin: 30px 0; }
		.otp-code { display: inline-block; font-size: 32px; font-weight: 700; letter-spacing: 5px; color: #2563eb; background-color: #eff6ff; padding: 15px 25px; border-radius: 6px; border: 1px dashed #bfdbfe; }
		.footer { background-color: #f8fafc; padding: 15px; text-align: center; font-size: 12px; color: #64748b; border-top: 1px solid #e2e8f0; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>Unicard</h1>
		</div>
		<div class="content">
			<p>Hello %s,</p>
			<p>We received a request to reset your password. Please use the following One-Time Password (OTP) to proceed. This OTP is valid for <strong>5 minutes</strong>.</p>
			<div class="otp-container">
				<div class="otp-code">%s</div>
			</div>
			<p>If you did not request a password reset, please ignore this email or contact support if you have concerns.</p>
			<p>Thank you,<br>The Unicard Team</p>
		</div>
		<div class="footer">
			&copy; ` + Year() + ` Unicard. All rights reserved.
		</div>
	</div>
</body>
</html>`
}

func Year() string {
	return fmt.Sprintf("%d", time.Now().Year())
}

func MerchantApprovedEmail() string {
	return `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Unicard Application Approved</title>
	<style>
		body { font-family: 'Inter', Arial, sans-serif; background-color: #f4f4f5; margin: 0; padding: 0; }
		.container { max-width: 600px; margin: 40px auto; background-color: #ffffff; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); overflow: hidden; }
		.header { background-color: #0f172a; color: #ffffff; padding: 20px; text-align: center; }
		.header h1 { margin: 0; font-size: 24px; font-weight: 600; }
		.content { padding: 30px; color: #334155; line-height: 1.6; }
		.content p { margin: 0 0 15px; }
		.btn-container { text-align: center; margin: 30px 0; }
		.btn { display: inline-block; background-color: #2563eb; color: #ffffff; text-decoration: none; padding: 12px 24px; border-radius: 6px; font-weight: 600; }
		.footer { background-color: #f8fafc; padding: 15px; text-align: center; font-size: 12px; color: #64748b; border-top: 1px solid #e2e8f0; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>Unicard</h1>
		</div>
		<div class="content">
			<p>Hello %s,</p>
			<p>Great news! Your merchant application has been verified and approved.</p>
			<p>You can now log in to your Unicard dashboard to set up your settlement bank information and start accepting payments.</p>
			<div class="btn-container">
				<a href="%s" class="btn">Login to Unicard</a>
			</div>
			<p>If you have any questions, feel free to contact our support team.</p>
			<p>Thank you,<br>The Unicard Team</p>
		</div>
		<div class="footer">
			&copy; ` + Year() + ` Unicard. All rights reserved.
		</div>
	</div>
</body>
</html>`
}

func MerchantRejectedEmail() string {
	return `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Unicard Application Update</title>
	<style>
		body { font-family: 'Inter', Arial, sans-serif; background-color: #f4f4f5; margin: 0; padding: 0; }
		.container { max-width: 600px; margin: 40px auto; background-color: #ffffff; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); overflow: hidden; }
		.header { background-color: #0f172a; color: #ffffff; padding: 20px; text-align: center; }
		.header h1 { margin: 0; font-size: 24px; font-weight: 600; }
		.content { padding: 30px; color: #334155; line-height: 1.6; }
		.content p { margin: 0 0 15px; }
		.reason-box { background-color: #fef2f2; border-left: 4px solid #ef4444; padding: 15px; margin: 20px 0; color: #7f1d1d; }
		.footer { background-color: #f8fafc; padding: 15px; text-align: center; font-size: 12px; color: #64748b; border-top: 1px solid #e2e8f0; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>Unicard</h1>
		</div>
		<div class="content">
			<p>Hello %s,</p>
			<p>Thank you for applying to become a Unicard merchant. Unfortunately, we are unable to approve your application at this time.</p>
			<div class="reason-box">
				<strong>Reason:</strong><br>
				%s
			</div>
			<p>If you believe this is an error or if you have the necessary information to address this issue, please contact our support team.</p>
			<p>Thank you,<br>The Unicard Team</p>
		</div>
		<div class="footer">
			&copy; ` + Year() + ` Unicard. All rights reserved.
		</div>
	</div>
</body>
</html>`
}

func MerchantSuspendedEmail() string {
	return `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Unicard Account Suspended</title>
	<style>
		body { font-family: 'Inter', Arial, sans-serif; background-color: #f4f4f5; margin: 0; padding: 0; }
		.container { max-width: 600px; margin: 40px auto; background-color: #ffffff; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); overflow: hidden; }
		.header { background-color: #0f172a; color: #ffffff; padding: 20px; text-align: center; }
		.header h1 { margin: 0; font-size: 24px; font-weight: 600; }
		.content { padding: 30px; color: #334155; line-height: 1.6; }
		.content p { margin: 0 0 15px; }
		.reason-box { background-color: #fff7ed; border-left: 4px solid #f97316; padding: 15px; margin: 20px 0; color: #9a3412; }
		.footer { background-color: #f8fafc; padding: 15px; text-align: center; font-size: 12px; color: #64748b; border-top: 1px solid #e2e8f0; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>Unicard</h1>
		</div>
		<div class="content">
			<p>Hello %s,</p>
			<p>This is a notice that your Unicard merchant account has been suspended.</p>
			<div class="reason-box">
				<strong>Reason for Suspension:</strong><br>
				%s
			</div>
			<p>During this suspension, you will not be able to log in or process transactions. If you wish to appeal this decision or require further clarification, please contact our support team immediately.</p>
			<p>Thank you,<br>The Unicard Team</p>
		</div>
		<div class="footer">
			&copy; ` + Year() + ` Unicard. All rights reserved.
		</div>
	</div>
</body>
</html>`
}

func EmailVerificationBody() string {
	return `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Approve Your Email Change</title>
	<style>
		body { font-family: 'Inter', Arial, sans-serif; background-color: #f4f4f5; margin: 0; padding: 0; }
		.container { max-width: 600px; margin: 40px auto; background-color: #ffffff; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); overflow: hidden; }
		.header { background-color: #0f172a; color: #ffffff; padding: 20px; text-align: center; }
		.header h1 { margin: 0; font-size: 24px; font-weight: 600; }
		.content { padding: 30px; color: #334155; line-height: 1.6; }
		.content p { margin: 0 0 15px; }
		.btn-container { text-align: center; margin: 30px 0; }
		.btn { display: inline-block; background-color: #2563eb; color: #ffffff; text-decoration: none; padding: 12px 24px; border-radius: 6px; font-weight: 600; }
		.footer { background-color: #f8fafc; padding: 15px; text-align: center; font-size: 12px; color: #64748b; border-top: 1px solid #e2e8f0; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>Unicard</h1>
		</div>
		<div class="content">
			<p>Hello %s,</p>
			<p>We received a request to update your Unicard email address to <strong>%s</strong>. To approve and confirm this change, please click the button below:</p>
			<div class="btn-container">
				<a href="%s" class="btn">Approve Email Change</a>
			</div>
			<p>If you did not request this change, you can safely ignore this email and your account will remain secure. Your email address will remain unchanged.</p>
			<p>Thank you,<br>The Unicard Team</p>
		</div>
		<div class="footer">
			&copy; ` + Year() + ` Unicard. All rights reserved.
	</div>
</body>
</html>`
}

func MerchantDeletedEmail() string {
	return `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Unicard Account Deleted</title>
	<style>
		body { font-family: 'Inter', Arial, sans-serif; background-color: #f4f4f5; margin: 0; padding: 0; }
		.container { max-width: 600px; margin: 40px auto; background-color: #ffffff; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); overflow: hidden; }
		.header { background-color: #0f172a; color: #ffffff; padding: 20px; text-align: center; }
		.header h1 { margin: 0; font-size: 24px; font-weight: 600; }
		.content { padding: 30px; color: #334155; line-height: 1.6; }
		.content p { margin: 0 0 15px; }
		.reason-box { background-color: #fef2f2; border-left: 4px solid #ef4444; padding: 15px; margin: 20px 0; color: #7f1d1d; }
		.footer { background-color: #f8fafc; padding: 15px; text-align: center; font-size: 12px; color: #64748b; border-top: 1px solid #e2e8f0; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>Unicard</h1>
		</div>
		<div class="content">
			<p>Hello %s,</p>
			<p>This is a formal notice that your Unicard merchant account has been permanently deleted by an administrator.</p>
			<div class="reason-box">
				<strong>Note:</strong><br>
				All terminals associated with your account have been deactivated and unregistered. Your account access has been revoked.
			</div>
			<p>If you have any pending settlements or questions regarding this action, please contact our support team immediately.</p>
			<p>Thank you,<br>The Unicard Team</p>
		</div>
		<div class="footer">
			&copy; ` + Year() + ` Unicard. All rights reserved.
		</div>
	</div>
</body>
</html>`
}

func PasswordChangedEmail() string {
	return `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Password Changed Successfully</title>
	<style>
		body { font-family: 'Inter', Arial, sans-serif; background-color: #f4f4f5; margin: 0; padding: 0; }
		.container { max-width: 600px; margin: 40px auto; background-color: #ffffff; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); overflow: hidden; }
		.header { background-color: #0f172a; color: #ffffff; padding: 20px; text-align: center; }
		.header h1 { margin: 0; font-size: 24px; font-weight: 600; }
		.content { padding: 30px; color: #334155; line-height: 1.6; }
		.content p { margin: 0 0 15px; }
		.btn-container { text-align: center; margin: 30px 0; }
		.btn { display: inline-block; background-color: #2563eb; color: #ffffff; text-decoration: none; padding: 12px 24px; border-radius: 6px; font-weight: 600; }
		.footer { background-color: #f8fafc; padding: 15px; text-align: center; font-size: 12px; color: #64748b; border-top: 1px solid #e2e8f0; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>Unicard</h1>
		</div>
		<div class="content">
			<p>Hello %s,</p>
			<p>Your password has been successfully changed.</p>
			<p>If you did not make this change, please contact our support immediately to secure your account.</p>
			<p>Thank you,<br>The Unicard Team</p>
		</div>
		<div class="footer">
			&copy; " + Year() + " Unicard. All rights reserved.
		</div>
	</div>
</body>
</html>`
}
